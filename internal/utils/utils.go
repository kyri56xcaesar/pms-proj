// Package utils provides a collection of reusable utility functions and helpers
// for use across the project. This package includes generic functional programming
// constructs (Map, Filter, Reduce), type conversion utilities, string manipulation,
// file operations, error formatting, and various validation helpers.
//
// Functional Programming Utilities:
//   - Map, Filter, Reduce: Generic implementations for slice processing.
//
// Type Conversion and Reflection:
//   - ToFloat64: Converts various numeric types to float64.
//   - IsEmpty: Checks if a value is the zero value for its type.
//   - MakeMapFrom: Constructs a map from two slices, omitting empty values.
//
// String Manipulation:
//   - ToSnakeCase: Converts CamelCase strings to snake_case.
//   - SplitToInt: Splits a string into a slice of integers.
//
// File Operations:
//   - MergeFiles: Concatenates multiple files into a single output file.
//   - ReadFileAt: Reads a specific byte range from a file.
//   - TailFileLines: Reads the last N lines from a file.
//
// Error Formatting:
//   - NewError, NewWarning, NewInfo: Formats error messages with severity tags.
//
// Slices:
//   - Contains
//
// Validation Helpers:
//   - HasInvalidCharacters: Checks for invalid characters in a string.
//   - IsNumeric, IsAlphanumeric, IsAlphanumericPlus: Validates string content.
//   - IsValidUTF8String: Validates if a string contains only allowed UTF-8 characters.
//
// Miscellaneous:
//   - CurrentTime: Returns the current UTC time in a standard format.
//   - SizeInGb: Converts a size in bytes to gigabytes.
//
// This package is intended to centralize commonly used logic and promote code reuse
// throughout the project.
package utils

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

/*
* This module will contain functions and methods useful for the other apps in the entire project
*
* Whatever is and can be reusable should be included here.
* */

/* some Functional Programming in Go */
// map
type mapFunc[E any, R any] func(E) R

// Map function definition of a functional programming "function"
func Map[S ~[]E, E any, R any](s S, f mapFunc[E, R]) []R {
	result := make([]R, len(s))
	for i, e := range s {
		result[i] = f(e)
	}

	return result
}

// filter
type keepFunc[E any] func(E) bool

// Filter function definition of a functional programming "function"
func Filter[S ~[]E, E any](s S, f keepFunc[E]) S {
	result := S{}
	for _, v := range s {
		if f(v) {
			result = append(result, v)
		}
	}

	return result
}

// reduce
type reduceFunc[E any] func(cur, next E) E

// Reduce function definition of a functional programming "function"
func Reduce[E any](s []E, init E, f reduceFunc[E]) E {
	cur := init
	for _, v := range s {
		cur = f(cur, v)
	}

	return cur
}

// util

// ToFloat64 function take any argument and tries to return a float64 if fail, 0
func ToFloat64(value any) float64 {
	switch v := value.(type) {
	case int:

		return float64(v)
	case int8:

		return float64(v)
	case int16:

		return float64(v)
	case int32:

		return float64(v)
	case int64:

		return float64(v)
	case uint:

		return float64(v)
	case uint8:

		return float64(v)
	case uint16:

		return float64(v)
	case uint32:

		return float64(v)
	case uint64:

		return float64(v)
	case float32:

		return float64(v)
	case float64:

		return v
	default:

		return 0
	}
}

// helper function to determine if a value is empty
func isZeroValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.String:

		return v.String() == ""
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:

		return v.Int() == 0
	case reflect.Array, reflect.Slice, reflect.Map:

		return v.Len() == 0
	case reflect.Bool:

		return !v.Bool()
	case reflect.Ptr, reflect.Interface:

		return v.IsNil()
	case reflect.Struct:
		// Recursively check each field
		for i := range v.NumField() {
			if !isZeroValue(v.Field(i)) {
				return false
			}
		}

		return true
	default:

		return v.IsZero() // general case for other types
		// this might not work for everything
	}
}

// IsEmpty will check if a given argument is "empty" using the reflect package
func IsEmpty(val any) bool {
	if val == nil {
		return true
	}

	return isZeroValue(reflect.ValueOf(val))
}

// MakeMapFrom will create a map string to any from given arguments, slices of string and any
func MakeMapFrom(names []string, values []any) map[string]any {
	if len(names) != len(values) {
		return nil
	}

	m := make(map[string]any)
	for i, arg := range values {
		reflectV := reflect.ValueOf(arg)
		if !IsEmpty(reflectV) {
			m[names[i]] = arg
		}
	}

	return m
}

/* generic helpers*/

// ToSnakeCase will format a given string to snake case
func ToSnakeCase(input string) string {
	output := make([]rune, 0, len(input))
	for i, r := range input {
		if i > 0 && r >= 'A' && r <= 'Z' {
			output = append(output, '_')
		}
		output = append(output, r)
	}

	return strings.ToLower(string(output))
}

// SplitToInt will peform and join string split and atoi
func SplitToInt(input, separator string) ([]int, error) {
	// split the input string by comma
	parts := strings.Split(input, separator)

	// trim spaces and convert to int
	trimAndConvert := func(s string) (int, error) {
		trimmed := strings.TrimSpace(s)

		return strconv.Atoi(trimmed)
	}

	// apply the trimAndConvert function to each part
	result := make([]int, len(parts))
	for i, part := range parts {
		value, err := trimAndConvert(part)
		if err != nil {
			return nil, err
		}
		result[i] = value
	}

	return result, nil
}

func SplitToInt64(input, seperator string) ([]int64, error) {
	// split the input by the seperator
	parts := strings.Split(input, seperator)

	// trim spaces and parse int64
	trimAndConvert := func(s string) (int64, error) {
		return strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	}

	result := make([]int64, len(parts))
	for i, part := range parts {
		value, err := trimAndConvert(part)
		if err != nil {
			return nil, err
		}
		result[i] = value
	}

	return result, nil
}

// MergeFiles will read the input files and create 1 output with the inputs appended
// Mux Many to 1
func MergeFiles(outputFile string, inputLocation string, inputFiles []string) error {
	out, err := os.Create(outputFile)
	if err != nil {
		return err
	}

	for _, inpPath := range inputFiles {
		fmt.Println("Processing:", inpPath)

		inpFile, err := os.Open(inputLocation + inpPath)
		if err != nil {
			fmt.Println("Error opening file:", inpPath, err)

			continue
		}

		_, err = io.Copy(out, inpFile) // Append content
		if err != nil {
			fmt.Println("Error writing file:", err)

			return err
		}

		_, err = out.WriteString("\n")
		if err != nil {
			fmt.Println("error writing string: ", err)

			return err
		}
		err = inpFile.Close()
		if err != nil {
			fmt.Printf("failed to close the input file: %v", err)

			return err
		}
	}

	err = out.Close()
	if err != nil {
		fmt.Printf("faild to close the file : %v", err)

		return err
	}

	fmt.Println("Merged files into:", outputFile)

	return nil
}

// short error messaging funcs..

// NewError as a wrapper function to fmt.Errorf with a format
func NewError(msg string, args ...any) error {
	return fmt.Errorf("[ERROR] %s", fmt.Sprintf(msg, args...))
}

// NewWarning as a wrapper function to fmt.Errorf with a format
func NewWarning(msg string, args ...any) error {
	return fmt.Errorf("[WARNING] %s", fmt.Sprintf(msg, args...))
}

// NewInfo as a wrapper function to fmt.Errorf with a format
func NewInfo(msg string, args ...any) error {
	return fmt.Errorf("[INFO] %s", fmt.Sprintf(msg, args...))
}

// ReadFileAt will read a specific file at specific window
func ReadFileAt(filePath string, start, end int64) ([]byte, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}

	fileInfo, err := file.Stat()
	if err != nil {
		return nil, err
	}
	fileSize := fileInfo.Size()

	if start >= fileSize {
		return nil, NewError("requested range exceeds file size")
	}
	if end >= fileSize {
		end = fileSize - 1
	}

	_, err = file.Seek(start, io.SeekStart)
	if err != nil {
		err1 := file.Close()
		if err1 != nil {
			return nil, err1
		}

		return nil, err
	}

	data := make([]byte, end-start+1)
	_, err = file.Read(data)
	if err != nil && !errors.Is(err, io.EOF) {
		err1 := file.Close()
		if err1 != nil {
			return nil, err1
		}

		return nil, err
	}

	err = file.Close()
	if err != nil {
		return nil, err
	}

	return data, nil
}

// TimeFormat string defines the desired format of Time that is mostly used in this system
var TimeFormat = "2006-01-02 15:04:05-07:00"

// CurrentTime uses the time package to retrieve the current time and formats it to the desired (specified above) string
func CurrentTime() string {
	return time.Now().UTC().Format(TimeFormat)
}

// HasInvalidCharacters function checks if a given string is within the regexp of the given chars
func HasInvalidCharacters(s, chars string) bool {
	// Escape regex meta-characters to avoid pattern errors
	escapedChars := make([]string, 0, len(chars))
	for _, c := range chars {
		escapedChars = append(escapedChars, regexp.QuoteMeta(string(c)))
	}
	pattern := "[" + strings.Join(escapedChars, "") + "]"
	re := regexp.MustCompile(pattern)

	return re.MatchString(s)
}

// Security related utils

// IsNumeric function checks if a given string matches the regex of numericals
func IsNumeric(s string) bool {
	re := regexp.MustCompile(`^[0-9]+$`)

	return re.MatchString(s)
}

// IsValidPath function checks if a given string is a valid "path" string
func IsValidPath(s string) bool {
	re := regexp.MustCompile(`^[a-zA-Z0-9._\-/]+$`)

	return re.MatchString(s)
}

// IsAlphanumeric function checks if the given string matches the regex of numericals and letter characters
func IsAlphanumeric(s string) bool {
	re := regexp.MustCompile(`^[a-zA-Z0-9]+$`)

	return re.MatchString(s)
}

// IsAlphanumericPlus function checks if the given string matches the regex of numericals
// and letter characters plus some special characters given
func IsAlphanumericPlus(s, plus string) bool {
	re := regexp.MustCompile(fmt.Sprintf(`^[a-zA-Z0-9%s]+$`, plus))

	return re.MatchString(s)
}

// IsAlphanumericPlusSome function checks if the given string matches the regex of numericals
// and letter characters plus some special characters already defined
func IsAlphanumericPlusSome(s string) bool {
	re := regexp.MustCompile(`^[a-zA-Z0-9@_.]+$`)

	return re.MatchString(s)
}

// IsValidUTF8String function will match the given string to a regex that describes only UTF-8 characters (no special)
func IsValidUTF8String(s string) bool {
	// Updated regex to include space (\s) and new line (\n) characters
	re := regexp.MustCompile(`^[\p{L}\p{N}\s\n!@#\$%\^&\*\(\):\?><\.\-_, ]+$`)

	return re.MatchString(s)
}

// SizeInGb function formats a int64 describing bytes to a float64 "gigabytes"
func SizeInGb(s int64) float64 {
	return float64(s) / 1000000000
}

// TailFileLines reads the last n lines from the file at the specified path.
// It efficiently seeks from the end of the file, reading backwards in blocks,
// and returns the last n lines as a slice of strings. If an error occurs
// during file operations, it is returned.
func TailFileLines(path string, n int) ([]string, error) {
	const readBlockSize = 4096

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		err := file.Close()
		if err != nil {
			fmt.Printf("failed to close file: %v", err)
		}
	}()

	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}

	var (
		fileSize  = stat.Size()
		buffer    bytes.Buffer
		lineCount = 0
		offset    int64
		tmp       = make([]byte, readBlockSize)
	)

	for offset = fileSize; offset > 0 && lineCount <= n; {
		blockSize := int64(readBlockSize)
		if offset < blockSize {
			blockSize = offset
		}
		offset -= blockSize
		_, err := file.Seek(offset, io.SeekStart)
		if err != nil {
			return nil, err
		}
		readBytes := tmp[:blockSize]
		nr, err := file.Read(readBytes)
		if err != nil {
			return nil, err
		}

		buffer.Write(readBytes[:nr])
		lineCount += bytes.Count(readBytes[:nr], []byte{'\n'})
	}

	content := buffer.Bytes()
	lines := make([]string, 0, n)
	scanner := bufio.NewScanner(bytes.NewReader(content))
	for scanner.Scan() {
		text := scanner.Text()
		if utf8.ValidString(text) {
			lines = append(lines, text)
		} else {
			lines = append(lines, string(bytes.Runes([]byte(text))))
		}
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}

	return lines, nil
}

// Contains function iterates over a slice of strings and checks if the given string is there
// if you want to avoid the slices.Contains package function
func Contains(slice []string, val string) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}

	return false
}

// ValidateObjectName sanitizes and formats + validates if a given name string argument is within limits
func ValidateObjectName(name string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("object name cannot be empty")
	}

	// if strings.Contains(name, "/") {
	// return errors.New("object name cannot contain slashes")
	// }

	if strings.Contains(name, "..") {
		return errors.New("object name cannot contain '..'")
	}

	// Optional: disallow special characters (Windows-style)
	illegalChars := regexp.MustCompile(`[\\:*?"<>|]`)
	if illegalChars.MatchString(name) {
		return errors.New("object name contains illegal characters")
	}

	// Optional: enforce max length
	if len(name) > 255 {
		return errors.New("object name is too long")
	}

	// Optional: ensure it has an extension
	// if !strings.Contains(name, ".") {
	// return errors.New("object name must include an extension (e.g., .txt)")
	// }

	return nil
}

func appendIfMissing(s, suffix string) string {
	if !strings.HasSuffix(s, suffix) {
		return s + suffix
	}

	return s
}

func parseMi(s string) (int, error) {
	if s == "" {
		return 0, nil
	}
	s = strings.TrimSuffix(s, "Mi")
	if s == "" {
		return 0, nil
	}

	return strconv.Atoi(s)
}

func parseGi(s string) (float64, error) {
	if s == "" {
		return 0, nil
	}
	s = strings.TrimSuffix(s, "Gi")
	if s == "" {
		return 0, nil
	}

	return strconv.ParseFloat(s, 64)
}

func parseCPU(s string) (float64, error) {
	if s == "" {
		return 0, nil
	}

	return strconv.ParseFloat(s, 64)
}

func GenerateRandomStringAll(length int) (string, error) {
	byteLength := (length * 6 / 8) + 1 // because base64 encodes 6 bits per character
	bytes := make([]byte, byteLength)

	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(bytes)[:length], nil
}

func GenerateRandomString(length int) (string, error) {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	bytes := make([]byte, length)
	random := make([]byte, length)
	_, err := rand.Read(random)
	if err != nil {
		return "", err
	}
	for i := 0; i < length; i++ {
		bytes[i] = chars[int(random[i])%len(chars)]
	}
	return string(bytes), nil
}
