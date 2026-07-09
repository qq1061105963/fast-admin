package rag

import (
	"math"
	"strconv"
	"strings"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/errs"
)

// splitText 复刻 Java 侧的切片逻辑：有分隔符按分隔符聚合、再补 overlap 前缀；无分隔符按长度滑窗。
func splitText(text string, chunkSize, overlap int, delimiter string) []string {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	var result []string
	if delimiter != "" {
		result = splitByDelimiter(text, chunkSize, delimiter)
	} else {
		result = splitByLength(text, chunkSize, overlap)
	}
	if delimiter == "" || overlap <= 0 || len(result) <= 1 {
		return result
	}
	overlapped := make([]string, 0, len(result))
	for i, chunk := range result {
		if i > 0 {
			prevRunes := []rune(result[i-1])
			start := len(prevRunes) - overlap
			if start < 0 {
				start = 0
			}
			prefix := strings.TrimSpace(string(prevRunes[start:]))
			if prefix != "" {
				chunk = prefix + "\n" + chunk
			}
		}
		overlapped = append(overlapped, chunk)
	}
	return overlapped
}

func splitByDelimiter(text string, chunkSize int, delimiter string) []string {
	var result []string
	parts := strings.Split(text, delimiter)
	var current strings.Builder
	flush := func() {
		v := strings.TrimSpace(current.String())
		if v != "" {
			result = append(result, v)
		}
		current.Reset()
	}
	for _, raw := range parts {
		part := strings.TrimSpace(raw)
		if part == "" {
			continue
		}
		if runeLen(part) > chunkSize {
			flush()
			result = append(result, splitByLength(part, chunkSize, 0)...)
			continue
		}
		var candidate string
		if current.Len() == 0 {
			candidate = part
		} else {
			candidate = current.String() + delimiter + part
		}
		if runeLen(candidate) <= chunkSize {
			current.Reset()
			current.WriteString(candidate)
		} else {
			flush()
			current.WriteString(part)
		}
	}
	flush()
	if len(result) == 0 {
		return splitByLength(text, chunkSize, 0)
	}
	return result
}

// splitByLength 按 rune 长度滑窗，避免在多字节字符中间切断（Java 用 char 索引，
// 这里用 rune 更安全，语义等价于按字符切）。
func splitByLength(text string, chunkSize, overlap int) []string {
	runes := []rune(text)
	var result []string
	start := 0
	for start < len(runes) {
		end := start + chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		chunk := strings.TrimSpace(string(runes[start:end]))
		if chunk != "" {
			result = append(result, chunk)
		}
		if end >= len(runes) {
			break
		}
		next := end - overlap
		if next < start+1 {
			next = start + 1
		}
		start = next
	}
	return result
}

func validateKB(dto *KBSaveDTO) error {
	if dto == nil {
		return errs.New(40117, 400, "知识库不能为空")
	}
	if strings.TrimSpace(dto.Name) == "" {
		return errs.New(40118, 400, "知识库名称不能为空")
	}
	chunkSize := defaultChunkSize
	if dto.ChunkSize != nil {
		chunkSize = *dto.ChunkSize
	}
	overlap := defaultChunkOverlap
	if dto.ChunkOverlap != nil {
		overlap = *dto.ChunkOverlap
	}
	if chunkSize < 100 || chunkSize > maxChunkSize {
		return errs.New(40119, 400, "切片长度需在 100~"+itoa(maxChunkSize)+" 之间")
	}
	if overlap < 0 || overlap >= chunkSize {
		return errs.New(40120, 400, "切片重叠需大于等于 0 且小于切片长度")
	}
	if strings.TrimSpace(dto.ChunkDelimiter) != "" && len(decodeDelimiter(dto.ChunkDelimiter)) > 100 {
		return errs.New(40121, 400, "切片分隔符长度不能超过 100")
	}
	return nil
}

func copyToKB(dto *KBSaveDTO, kb *AiKnowledgeBase) {
	kb.Name = dto.Name
	kb.Description = dto.Description
	if dto.Enabled != nil {
		kb.Enabled = *dto.Enabled
	}
	if dto.ChunkSize != nil {
		kb.ChunkSize = *dto.ChunkSize
	} else {
		kb.ChunkSize = defaultChunkSize
	}
	if dto.ChunkOverlap != nil {
		kb.ChunkOverlap = *dto.ChunkOverlap
	} else {
		kb.ChunkOverlap = defaultChunkOverlap
	}
	if strings.TrimSpace(dto.ChunkDelimiter) != "" {
		kb.ChunkDelimiter = strings.TrimSpace(dto.ChunkDelimiter)
	} else {
		kb.ChunkDelimiter = defaultChunkDelim
	}
	kb.Remark = dto.Remark
}

func kbChunkSize(kb *AiKnowledgeBase) int {
	if kb.ChunkSize == 0 {
		return defaultChunkSize
	}
	return kb.ChunkSize
}

func kbChunkOverlap(kb *AiKnowledgeBase) int {
	if kb.ChunkOverlap == 0 {
		return defaultChunkOverlap
	}
	return kb.ChunkOverlap
}

func kbChunkDelimiter(kb *AiKnowledgeBase) string {
	delim := kb.ChunkDelimiter
	if strings.TrimSpace(delim) == "" {
		delim = defaultChunkDelim
	}
	return decodeDelimiter(delim)
}

// decodeDelimiter 把字面量转义序列解码成真实字符，对齐 Java 的 decodeDelimiter。
func decodeDelimiter(delimiter string) string {
	r := strings.NewReplacer(`\r`, "\r", `\n`, "\n", `\t`, "\t")
	return r.Replace(delimiter)
}

func estimateTokens(text string) int {
	n := int(math.Ceil(float64(len([]rune(text))) / 4.0))
	if n < 1 {
		return 1
	}
	return n
}

func clampRag(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func itoa(v int) string { return strconv.Itoa(v) }

func runeLen(s string) int { return len([]rune(s)) }

func containsStr(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}
