package rag

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"

	"github.com/xuri/excelize/v2"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/errs"
)

// extractText 从文档字节里提取纯文本，对应 Java 侧的 AiDocumentTextExtractor。
// 纯文本类、docx、pptx、xls/xlsx 均支持；旧版二进制 doc/ppt（OLE 复合文档）在 Go 侧
// 缺少成熟解析库，暂不支持并给出明确提示。
func extractText(ext string, data []byte) (string, error) {
	ext = strings.ToLower(strings.TrimSpace(ext))
	if ext == "" {
		return "", errs.New(40100, 400, "文件扩展名为空，无法解析")
	}
	var (
		text string
		err  error
	)
	switch ext {
	case "txt", "md", "markdown", "csv", "json", "xml", "html", "log", "yml", "yaml":
		text = string(data)
	case "docx":
		text, err = extractDocx(data)
	case "pptx":
		text, err = extractPptx(data)
	case "xls", "xlsx":
		text, err = extractWorkbook(data)
	case "doc", "ppt":
		return "", errs.New(40101, 400, "暂不支持解析旧版二进制 ."+ext+" 文件，请另存为 .docx/.pptx 后再上传")
	default:
		return "", errs.New(40102, 400, "暂不支持解析 ."+ext+
			" 文件，请上传 txt/md/csv/json/xml/html/yml/docx/pptx/xls/xlsx 等文件")
	}
	if err != nil {
		if _, ok := err.(*errs.AppError); ok {
			return "", err
		}
		return "", errs.New(40103, 400, "读取文档失败："+err.Error())
	}
	normalized := normalizeText(text)
	if strings.TrimSpace(normalized) == "" {
		return "", errs.New(40104, 400, "文档没有可索引文本")
	}
	return normalized, nil
}

var multiNewline = regexp.MustCompile(`\n{3,}`)

func normalizeText(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = multiNewline.ReplaceAllString(text, "\n\n")
	return strings.TrimSpace(text)
}

// ---- docx：解压 zip 读取 word/document.xml，取所有 <w:t> 文本，按段落换行 ----

func extractDocx(data []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", err
	}
	for _, f := range zr.File {
		if f.Name != "word/document.xml" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", err
		}
		defer rc.Close()
		raw, err := io.ReadAll(rc)
		if err != nil {
			return "", err
		}
		return docxXMLToText(raw), nil
	}
	return "", errs.New(40105, 400, "docx 缺少 word/document.xml")
}

// docxXMLToText 顺序解析 XML：<w:t> 累积文本，<w:p> 结束换行，<w:tab> 转空格。
func docxXMLToText(raw []byte) string {
	dec := xml.NewDecoder(bytes.NewReader(raw))
	var sb strings.Builder
	var line strings.Builder
	inText := false
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "t":
				inText = true
			case "tab":
				line.WriteByte(' ')
			case "br":
				line.WriteByte('\n')
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "t":
				inText = false
			case "p":
				appendLine(&sb, line.String())
				line.Reset()
			}
		case xml.CharData:
			if inText {
				line.Write(t)
			}
		}
	}
	if strings.TrimSpace(line.String()) != "" {
		appendLine(&sb, line.String())
	}
	return sb.String()
}

// ---- pptx：遍历 ppt/slides/slideN.xml，取 <a:t> 文本 ----

func extractPptx(data []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", err
	}
	slideRe := regexp.MustCompile(`^ppt/slides/slide(\d+)\.xml$`)
	type slide struct {
		idx int
		raw []byte
	}
	var slides []slide
	for _, f := range zr.File {
		m := slideRe.FindStringSubmatch(f.Name)
		if m == nil {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", err
		}
		raw, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return "", err
		}
		idx := 0
		fmt.Sscanf(m[1], "%d", &idx)
		slides = append(slides, slide{idx: idx, raw: raw})
	}
	sort.Slice(slides, func(i, j int) bool { return slides[i].idx < slides[j].idx })
	var sb strings.Builder
	for i, s := range slides {
		appendLine(&sb, fmt.Sprintf("第 %d 页", i+1))
		appendLine(&sb, pptxXMLToText(s.raw))
	}
	return sb.String(), nil
}

func pptxXMLToText(raw []byte) string {
	dec := xml.NewDecoder(bytes.NewReader(raw))
	var sb strings.Builder
	inText := false
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "t" {
				inText = true
			}
		case xml.EndElement:
			if t.Name.Local == "t" {
				inText = false
			}
		case xml.CharData:
			if inText {
				appendLine(&sb, string(t))
			}
		}
	}
	return sb.String()
}

// ---- xls/xlsx：excelize 逐 sheet 逐行读取 ----

func extractWorkbook(data []byte) (string, error) {
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	defer f.Close()
	var sb strings.Builder
	for _, sheet := range f.GetSheetList() {
		appendLine(&sb, "Sheet: "+sheet)
		rows, err := f.GetRows(sheet)
		if err != nil {
			continue
		}
		for i, row := range rows {
			var values []string
			for _, cell := range row {
				v := strings.TrimSpace(cell)
				if v != "" {
					values = append(values, v)
				}
			}
			if len(values) > 0 {
				appendLine(&sb, fmt.Sprintf("第 %d 行: %s", i+1, strings.Join(values, " | ")))
			}
		}
	}
	return sb.String(), nil
}

func appendLine(sb *strings.Builder, value string) {
	v := strings.TrimSpace(value)
	if v == "" {
		return
	}
	sb.WriteString(v)
	sb.WriteByte('\n')
}
