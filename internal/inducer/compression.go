package inducer

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// NumberCompressor implements number range compression for regex patterns
// This converts verbose alternations like (01|02|03|04|05) into compact ranges like [0][1-5]
// Following the regulator compress_number_ranges algorithm
type NumberCompressor struct {
	// Pattern to match number alternations: (num1|num2|num3)
	numberGroupPattern *regexp.Regexp
}

// NewNumberCompressor creates a new number compressor
func NewNumberCompressor() *NumberCompressor {
	return &NumberCompressor{
		// Match groups containing pipe-separated numbers (with optional dash prefix)
		numberGroupPattern: regexp.MustCompile(`\(([-0-9\|]+)\)`),
	}
}

// Compress applies number range compression to a regex pattern
// Returns the optimized pattern
func (nc *NumberCompressor) Compress(pattern string) string {
	result := pattern

	// Find all number groups
	matches := nc.numberGroupPattern.FindAllStringSubmatch(pattern, -1)

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}

		fullMatch := match[0] // e.g., "(01|02|03)"
		innerMatch := match[1] // e.g., "01|02|03"

		// Check if this is a pure number group
		if !nc.isNumberGroup(innerMatch) {
			continue
		}

		// Extract numbers
		numbers := strings.Split(innerMatch, "|")

		// Try to compress
		compressed := nc.compressNumbers(numbers)

		if compressed != "" && compressed != fullMatch {
			// Replace in result
			result = strings.ReplaceAll(result, fullMatch, compressed)
		}
	}

	return result
}

// isNumberGroup checks if a group contains only numbers
func (nc *NumberCompressor) isNumberGroup(group string) bool {
	parts := strings.Split(group, "|")

	for _, part := range parts {
		// Check if part is a valid number (possibly with leading dash)
		trimmed := strings.TrimPrefix(part, "-")
		if trimmed == "" {
			return false
		}

		// Check if it's all digits
		for _, ch := range trimmed {
			if ch < '0' || ch > '9' {
				return false
			}
		}
	}

	return true
}

// compressNumbers converts a list of numbers into a compact range representation
// Examples:
//   - ["01", "02", "03"] -> "[0][1-3]"
//   - ["1", "2", "3"] -> "[1-3]"
//   - ["-01", "-02", "-03"] -> "(-[0][1-3])"
func (nc *NumberCompressor) compressNumbers(numbers []string) string {
	if len(numbers) <= 1 {
		return "" // Can't compress single number
	}

	// Check if all numbers have dash prefix
	hasDash := strings.HasPrefix(numbers[0], "-")
	for _, num := range numbers {
		if strings.HasPrefix(num, "-") != hasDash {
			// Mixed - can't compress uniformly
			return ""
		}
	}

	// Strip dashes if present
	stripped := make([]string, len(numbers))
	for i, num := range numbers {
		stripped[i] = strings.TrimPrefix(num, "-")
	}

	// Convert to integers and sort
	intNums := make([]int, 0, len(stripped))
	for _, num := range stripped {
		val, err := strconv.Atoi(num)
		if err != nil {
			return "" // Invalid number
		}
		intNums = append(intNums, val)
	}
	sort.Ints(intNums)

	// Determine max width (for leading zeros)
	maxWidth := len(stripped[0])
	for _, num := range stripped {
		if len(num) > maxWidth {
			maxWidth = len(num)
		}
	}

	// Try to compress by digit position
	compressed := nc.compressByDigitPosition(intNums, maxWidth)

	if compressed == "" {
		// Fall back to simple range if sequential
		if nc.isSequential(intNums) {
			min := intNums[0]
			max := intNums[len(intNums)-1]
			compressed = nc.formatSimpleRange(min, max, maxWidth)
		}
	}

	if compressed == "" {
		return "" // Compression failed
	}

	// Add dash prefix back if needed
	if hasDash {
		compressed = "(-" + compressed + ")"
	} else {
		compressed = "(" + compressed + ")"
	}

	return compressed
}

// compressByDigitPosition analyzes numbers by digit position
// Returns a compact range like [0-1][0-9] or [1-3]
func (nc *NumberCompressor) compressByDigitPosition(numbers []int, width int) string {
	if len(numbers) == 0 {
		return ""
	}

	// Format all numbers with leading zeros to same width
	formatted := make([]string, len(numbers))
	for i, num := range numbers {
		formatted[i] = nc.formatWithWidth(num, width)
	}

	// Analyze each digit position
	positions := make([]map[rune]bool, width)
	for i := 0; i < width; i++ {
		positions[i] = make(map[rune]bool)
	}

	for _, numStr := range formatted {
		for i, digit := range numStr {
			positions[i][digit] = true
		}
	}

	// Build range for each position
	rangeParts := make([]string, width)
	for i, digitSet := range positions {
		// Convert to sorted slice
		digits := make([]rune, 0, len(digitSet))
		for digit := range digitSet {
			digits = append(digits, digit)
		}
		sort.Slice(digits, func(a, b int) bool { return digits[a] < digits[b] })

		if len(digits) == 1 {
			// Single digit - no range needed
			rangeParts[i] = string(digits[0])
		} else if nc.isConsecutiveDigits(digits) {
			// Consecutive - use range
			rangeParts[i] = "[" + string(digits[0]) + "-" + string(digits[len(digits)-1]) + "]"
		} else {
			// Non-consecutive - use alternation
			rangeParts[i] = "[" + string(digits) + "]"
		}
	}

	return strings.Join(rangeParts, "")
}

// formatWithWidth formats a number with leading zeros to a specific width
func (nc *NumberCompressor) formatWithWidth(num, width int) string {
	str := strconv.Itoa(num)
	for len(str) < width {
		str = "0" + str
	}
	return str
}

// isSequential checks if numbers form a sequential sequence
func (nc *NumberCompressor) isSequential(numbers []int) bool {
	if len(numbers) < 2 {
		return false
	}

	for i := 1; i < len(numbers); i++ {
		if numbers[i] != numbers[i-1]+1 {
			return false
		}
	}

	return true
}

// isConsecutiveDigits checks if a slice of digit runes is consecutive
func (nc *NumberCompressor) isConsecutiveDigits(digits []rune) bool {
	if len(digits) < 2 {
		return false
	}

	for i := 1; i < len(digits); i++ {
		if digits[i] != digits[i-1]+1 {
			return false
		}
	}

	return true
}

// formatSimpleRange formats a simple sequential range
// e.g., 1-9 -> [1-9], 01-09 -> [0][1-9]
func (nc *NumberCompressor) formatSimpleRange(min, max, width int) string {
	if width == 1 {
		// Single digit range
		return "[" + strconv.Itoa(min) + "-" + strconv.Itoa(max) + "]"
	}

	// Multi-digit - use digit position analysis
	return nc.compressByDigitPosition(nc.rangeToSlice(min, max), width)
}

// rangeToSlice converts a range to a slice of integers
func (nc *NumberCompressor) rangeToSlice(min, max int) []int {
	result := make([]int, max-min+1)
	for i := range result {
		result[i] = min + i
	}
	return result
}

// CompressPattern is a convenience method that compresses a Pattern's regex
func (nc *NumberCompressor) CompressPattern(pattern *Pattern) {
	if pattern == nil {
		return
	}

	compressed := nc.Compress(pattern.Regex)
	pattern.Regex = compressed
}
