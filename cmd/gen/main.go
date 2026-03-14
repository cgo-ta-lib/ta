// cmd/gen clones the TA-Lib repository at a pinned version, builds the amalgamation
// C file, copies headers, and generates Go wrapper source files.
//
// Usage: go run ./cmd/gen <version-tag>
// Example: go run ./cmd/gen v0.6.4
package main

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"fmt"
	"go/format"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: go run ./cmd/gen <version-tag>")
		os.Exit(1)
	}
	version := os.Args[1]

	// Determine repository root by walking up from the working directory to find go.mod.
	// go generate sets the working directory to the package directory, so cwd is the repo root.
	repoRoot, err := os.Getwd()
	must(err)
	for {
		if _, err := os.Stat(filepath.Join(repoRoot, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(repoRoot)
		if parent == repoRoot {
			fmt.Fprintln(os.Stderr, "could not find module root (no go.mod found)")
			os.Exit(1)
		}
		repoRoot = parent
	}

	fmt.Printf("cgo-ta-lib gen: version=%s root=%s\n", version, repoRoot)

	// Step 1: Clone TA-Lib into a temp directory.
	tmpDir, err := os.MkdirTemp("", "ta-lib-clone-*")
	must(err)
	defer os.RemoveAll(tmpDir)

	fmt.Printf("Cloning TA-Lib %s into %s...\n", version, tmpDir)
	runCmd("git", "clone", "--depth=1", "--branch", version,
		"https://github.com/TA-Lib/ta-lib", tmpDir)

	// Step 2: Run cmake to generate ta_config.h.
	buildDir := filepath.Join(tmpDir, "build")
	must(os.MkdirAll(buildDir, 0o755))
	fmt.Println("Running cmake configure...")
	runCmd("cmake", "-S", tmpDir, "-B", buildDir)

	// Step 3: Copy headers.
	// include/ gets public headers + all internal headers from src/ subdirectories.
	// The amalgamation uses #include "internal_header.h" (unqualified), so all
	// headers must be findable from a single -I path.
	includeDir := filepath.Join(repoRoot, "include")
	must(os.MkdirAll(includeDir, 0o755))

	copyDir(filepath.Join(tmpDir, "include"), includeDir)

	// Copy internal headers from each src/ subdirectory.
	for _, sub := range []string{"ta_common", "ta_func", "ta_abstract"} {
		copyDirGlob(filepath.Join(tmpDir, "src", sub), includeDir, ".h")
	}

	// Copy ta_config.h from cmake build output.
	cmakeInclude := filepath.Join(buildDir, "include")
	if _, err := os.Stat(cmakeInclude); err == nil {
		copyDir(cmakeInclude, includeDir)
	} else {
		// Some versions place it directly in the build dir.
		if src, err := os.ReadFile(filepath.Join(buildDir, "ta_config.h")); err == nil {
			must(os.WriteFile(filepath.Join(includeDir, "ta_config.h"), src, 0o644))
		} else {
			fmt.Fprintln(os.Stderr, "warning: could not find ta_config.h from cmake output")
		}
	}

	// Step 4: Build amalgamation.
	fmt.Println("Building amalgamation...")
	amalgPath := filepath.Join(repoRoot, "talib_amalgamation.c")
	buildAmalgamation(tmpDir, amalgPath, version)

	// Step 5: Parse ta_retcode.csv and write errors_gen.go.
	fmt.Println("Generating errors_gen.go...")
	csvPath := filepath.Join(tmpDir, "ta_retcode.csv")
	// Some versions embed it elsewhere; try a few locations.
	if _, err := os.Stat(csvPath); os.IsNotExist(err) {
		csvPath = filepath.Join(tmpDir, "src", "ta_common", "ta_retcode.csv")
	}
	retCodes := parseRetCodeCSV(csvPath)
	writeErrorsGen(filepath.Join(repoRoot, "errors_gen.go"), version, retCodes)

	// Step 6 & 7: Parse ta_func.h and extract doc comments from .c files.
	fmt.Println("Parsing ta_func.h...")
	taFuncH := filepath.Join(includeDir, "ta_func.h")
	funcDescs := parseTaFuncH(taFuncH)

	extractDocComments(funcDescs, amalgPath)

	// Step 8: Generate functions_gen.go.
	fmt.Println("Generating functions_gen.go...")
	writeFunctionsGen(filepath.Join(repoRoot, "functions_gen.go"), version, funcDescs)

	// Step 9: Generate lookback_gen.go.
	fmt.Println("Generating lookback_gen.go...")
	writeLookbackGen(filepath.Join(repoRoot, "lookback_gen.go"), version, funcDescs)

	fmt.Println("Done.")
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runCmd(name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "command %s %v failed: %v\n", name, args, err)
		os.Exit(1)
	}
}

func copyDir(src, dst string) {
	copyDirGlob(src, dst, "")
}

// copyDirGlob copies files from src to dst, optionally filtering by extension (e.g. ".h").
func copyDirGlob(src, dst, ext string) {
	entries, err := os.ReadDir(src)
	if err != nil {
		return // silently skip missing dirs
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if ext != "" && !strings.HasSuffix(e.Name(), ext) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(src, e.Name()))
		must(err)
		must(os.WriteFile(filepath.Join(dst, e.Name()), data, 0o644))
	}
}

// buildAmalgamation concatenates all C source files into a single file.
func buildAmalgamation(tmpDir, outPath, version string) {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "// Code generated by cmd/gen %s. DO NOT EDIT.\n\n", version)

	appendDir := func(label, dir, glob string) {
		entries, err := filepath.Glob(filepath.Join(dir, glob))
		if err != nil || len(entries) == 0 {
			fmt.Fprintf(os.Stderr, "warning: no files matched %s in %s\n", glob, dir)
			return
		}
		sort.Strings(entries)
		for _, path := range entries {
			fmt.Fprintf(&buf, "// --- %s ---\n", filepath.Base(path))
			data, err := os.ReadFile(path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not read %s: %v\n", path, err)
				continue
			}
			buf.Write(data)
			buf.WriteByte('\n')
		}
		_ = label
	}

	appendDir("ta_common", filepath.Join(tmpDir, "src", "ta_common"), "*.c")
	appendDir("ta_func", filepath.Join(tmpDir, "src", "ta_func"), "ta_*.c")

	must(os.WriteFile(outPath, buf.Bytes(), 0o644))
}

// --- Error code parsing ---

type retCode struct {
	code int
	name string
	desc string
}

func parseRetCodeCSV(path string) []retCode {
	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not open %s: %v\n", path, err)
		return nil
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.Comment = '#'
	r.FieldsPerRecord = -1

	var codes []retCode
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil || len(record) < 3 {
			continue
		}
		var code int
		fmt.Sscanf(strings.TrimSpace(record[0]), "%d", &code)
		name := strings.TrimSpace(record[1])
		desc := strings.TrimSpace(record[2])
		codes = append(codes, retCode{code, name, desc})
	}
	return codes
}

func writeErrorsGen(path, version string, codes []retCode) {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "// Code generated by cmd/gen %s. DO NOT EDIT.\n\npackage ta\n\n", version)
	buf.WriteString("var retCodeMessages = map[int]string{\n")
	for _, c := range codes {
		fmt.Fprintf(&buf, "\t%d: %q,\n", c.code, fmt.Sprintf("%s(%d): %s", c.name, c.code, c.desc))
	}
	buf.WriteString("}\n")

	out, err := format.Source(buf.Bytes())
	if err != nil {
		// Write unformatted but warn.
		fmt.Fprintln(os.Stderr, "warning: errors_gen.go format failed:", err)
		out = buf.Bytes()
	}
	must(os.WriteFile(path, out, 0o644))
}

// --- ta_func.h parsing ---

// FuncDescriptor holds everything needed to generate one wrapper function.
type FuncDescriptor struct {
	Name        string   // e.g. "ADX"
	GoName      string   // e.g. "Adx"
	Inputs      []Param  // double[] or integer[] inputs
	OptIns      []Param  // optional integer/double/MAType params
	Outputs     []Param  // double[] or integer[] outputs
	LookbackSig string   // raw C parameter list for Lookback, e.g. "int optInTimePeriod"
	DocComment  string   // extracted from .c file
}

type Param struct {
	CType  string // "double", "int", "TA_Integer", "TA_MAType"
	Name   string // e.g. "inHigh", "optInTimePeriod", "outReal"
	GoName string // camelCase Go name
}

var (
	// TA_LIB_API prefix is present in v0.6.x headers. Match both with and without it.
	// Skip TA_S_ variants (single-precision float versions of each function).
	reFuncStart     = regexp.MustCompile(`(?:TA_LIB_API\s+)?TA_RetCode\s+TA_([A-Z0-9_]+)\s*\(`)
	reLookbackStart = regexp.MustCompile(`(?:TA_LIB_API\s+)?int\s+TA_([A-Z0-9_]+?)_Lookback\s*\(([^)]*)\)`)
	reParamLine     = regexp.MustCompile(`^\s*(const\s+)?(double|TA_Real|TA_Integer|int|TA_MAType)\s+[\*]?(in|optIn|out)([A-Za-z0-9_]+)`)
	// reDocLine matches the function-description line inside a GENCODE SECTION 3 comment block.
	reDocLine = regexp.MustCompile(`^\s*\*\s+TA_([A-Z0-9_]+)\s+-\s+`)
	// reOptIn matches optIn-prefixed parameter names in doc comment text.
	reOptIn = regexp.MustCompile(`optIn([A-Z][A-Za-z0-9]*)`)
)

// outParamSkip are internal TA-Lib output parameters that are not data arrays.
var outParamSkip = map[string]bool{"BegIdx": true, "NBElement": true}

func parseTaFuncH(path string) []*FuncDescriptor {
	f, err := os.Open(path)
	must(err)
	defer f.Close()

	byName := map[string]*FuncDescriptor{}
	var order []string

	scanner := bufio.NewScanner(f)
	var current *FuncDescriptor
	inFunc := false

	for scanner.Scan() {
		line := scanner.Text()

		// Lookback signature — always single line in ta_func.h.
		if m := reLookbackStart.FindStringSubmatch(line); m != nil {
			name := m[1]
			// Skip TA_S_ variants (single-precision).
			if strings.HasPrefix(name, "S_") {
				continue
			}
			sig := strings.TrimSpace(m[2])
			if d, ok := byName[name]; ok {
				d.LookbackSig = sig
			}
			continue
		}

		// Function start.
		if m := reFuncStart.FindStringSubmatch(line); m != nil {
			name := m[1]
			// Skip TA_S_ variants (single-precision).
			if strings.HasPrefix(name, "S_") {
				inFunc = false
				current = nil
				continue
			}
			if _, seen := byName[name]; !seen {
				d := &FuncDescriptor{
					Name:   name,
					GoName: toCamel(name),
				}
				byName[name] = d
				order = append(order, name)
			}
			current = byName[name]
			inFunc = true
			continue
		}

		if inFunc {
			trimmed := strings.TrimSpace(line)

			// Try to pick up a param from this line first (e.g. "double outReal[] );").
			if m := reParamLine.FindStringSubmatch(line); m != nil {
				rawType := m[2]
				prefix := m[3] // "in", "optIn", "out"
				suffix := m[4] // e.g. "High", "TimePeriod", "Real"

				if !outParamSkip[suffix] {
					rawName := prefix + suffix
					p := Param{
						CType:  normalizeCType(rawType),
						Name:   rawName,
						GoName: paramGoName(prefix, suffix),
					}
					switch prefix {
					case "in":
						current.Inputs = append(current.Inputs, p)
					case "optIn":
						current.OptIns = append(current.OptIns, p)
					case "out":
						current.Outputs = append(current.Outputs, p)
					}
				}
			}

			// End of function block: ); on its own line, or the last param line ends with );
			if strings.HasPrefix(trimmed, ");") || trimmed == ")" ||
				strings.HasSuffix(trimmed, ");") || strings.HasSuffix(trimmed, ")") {
				inFunc = false
				current = nil
			}
		}
	}
	must(scanner.Err())

	// Only keep functions that have at least one output (sanity filter).
	result := make([]*FuncDescriptor, 0, len(order))
	for _, name := range order {
		d := byName[name]
		if len(d.Outputs) > 0 {
			result = append(result, d)
		}
	}
	return result
}

func normalizeCType(raw string) string {
	switch raw {
	case "TA_Real":
		return "double"
	case "TA_Integer":
		return "int"
	default:
		return raw
	}
}

// toCamel converts "ADX" → "Adx", "MACD" → "Macd", "MINUS_DI" → "MinusDi".
func toCamel(s string) string {
	parts := strings.Split(s, "_")
	var b strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		b.WriteString(strings.ToUpper(p[:1]))
		b.WriteString(strings.ToLower(p[1:]))
	}
	return b.String()
}

// paramGoName converts prefix+suffix to a Go-idiomatic camelCase name.
// "in"+"High" → "high", "optIn"+"TimePeriod" → "timePeriod", "out"+"Real" → "outReal".
func paramGoName(prefix, suffix string) string {
	lower := strings.ToLower(suffix[:1]) + suffix[1:]
	switch prefix {
	case "in":
		// Rename common OHLCV names.
		switch lower {
		case "high":
			return "high"
		case "low":
			return "low"
		case "close", "closePrice":
			return "closePrice"
		case "open":
			return "open"
		case "volume":
			return "volume"
		case "real":
			return "in"
		case "real0":
			return "in0"
		case "real1":
			return "in1"
		default:
			return lower
		}
	case "optIn":
		return lower
	case "out":
		return "out" + strings.ToUpper(suffix[:1]) + suffix[1:]
	}
	return lower
}

// --- Doc comment extraction ---

// extractDocComments reads the amalgamation and finds the GENCODE SECTION 3 comment block
// for each function. These blocks contain the human-readable Input/Output/Parameters
// description and are identified by the pattern "* TA_FUNCNAME - description".
func extractDocComments(descs []*FuncDescriptor, amalgPath string) {
	data, err := os.ReadFile(amalgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not read amalgamation for doc comments: %v\n", err)
		return
	}
	comments := parseDocComments(string(data))
	for _, d := range descs {
		if c, ok := comments[d.Name]; ok {
			d.DocComment = transformDocComment(c, d)
		}
	}
}

// parseDocComments scans the amalgamation source for standalone /* ... */ blocks that
// contain a "* TA_FUNCNAME - description" line (the GENCODE SECTION 3 doc blocks).
// Returns a map from TA-Lib function name (e.g. "ACCBANDS") to cleaned comment body.
func parseDocComments(src string) map[string]string {
	result := map[string]string{}
	lines := strings.Split(src, "\n")
	n := len(lines)
	for i := 0; i < n; i++ {
		// Look for a standalone "/*" that opens a doc block.
		if strings.TrimSpace(lines[i]) != "/*" {
			continue
		}
		blockStart := i + 1
		funcName := ""
		j := blockStart
		for j < n {
			t := strings.TrimSpace(lines[j])
			if t == "*/" {
				break
			}
			if m := reDocLine.FindStringSubmatch(lines[j]); m != nil && funcName == "" {
				funcName = m[1]
			}
			j++
		}
		if funcName != "" {
			var cleaned []string
			for k := blockStart; k < j; k++ {
				cleaned = append(cleaned, cleanCommentLine(lines[k]))
			}
			result[funcName] = strings.Join(trimBlankLines(cleaned), "\n")
		}
		i = j // skip to closing */
	}
	return result
}

// cleanCommentLine strips the leading " * " or " *" from a C block comment line
// and trims any remaining leading/trailing whitespace.
func cleanCommentLine(line string) string {
	s := strings.TrimLeft(line, " \t")
	if after, ok := strings.CutPrefix(s, "* "); ok {
		return strings.TrimSpace(after)
	}
	return strings.TrimSpace(strings.TrimPrefix(s, "*"))
}

// trimBlankLines removes leading and trailing empty lines from a slice.
func trimBlankLines(lines []string) []string {
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// transformDocComment adjusts a raw TA-Lib doc comment for use as a Go doc comment:
//   - Replaces "TA_FUNCNAME - description" with "GoName - description" on the first line.
//   - Strips the "optIn" prefix from parameter names so they match the Go signature.
func transformDocComment(comment string, d *FuncDescriptor) string {
	lines := strings.Split(comment, "\n")
	if len(lines) > 0 {
		lines[0] = strings.Replace(lines[0], "TA_"+d.Name+" - ", d.GoName+" - ", 1)
	}
	for i, line := range lines {
		lines[i] = reOptIn.ReplaceAllStringFunc(line, func(match string) string {
			suffix := match[5:] // strip "optIn"
			return strings.ToLower(suffix[:1]) + suffix[1:]
		})
	}
	return strings.Join(lines, "\n")
}

// outBufParamName derives a short buffer parameter name from an output GoName.
// e.g. "outMACD" → "macdBuf", "outRealUpperBand" → "upperBandBuf", "outAroonDown" → "aroonDownBuf".
func outBufParamName(goName string) string {
	s := goName
	// Strip "out" prefix.
	s = strings.TrimPrefix(s, "out")
	// Strip "Real" prefix if followed by an uppercase letter (it's a filler word, not semantic).
	if strings.HasPrefix(s, "Real") && len(s) > 4 && s[4] >= 'A' && s[4] <= 'Z' {
		s = s[4:]
	}
	if s == "" {
		return goName + "Buf"
	}
	// Find the leading uppercase run.
	i := 0
	for i < len(s) && s[i] >= 'A' && s[i] <= 'Z' {
		i++
	}
	var base string
	switch {
	case i == 0:
		base = s
	case i == len(s):
		// All uppercase (e.g. "MACD") — fully lowercase.
		base = strings.ToLower(s)
	case i == 1:
		// Single leading uppercase (e.g. "UpperBand") — lowercase first char.
		base = strings.ToLower(s[:1]) + s[1:]
	default:
		// Mixed run (e.g. "MACDSignal", "MACDHist") — lowercase all but the last
		// uppercase char, which starts the next CamelCase word.
		base = strings.ToLower(s[:i-1]) + s[i-1:]
	}
	return base + "Buf"
}

// --- Code generation ---

func writeFunctionsGen(path, version string, descs []*FuncDescriptor) {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "// Code generated by cmd/gen %s. DO NOT EDIT.\n\npackage ta\n\n", version)
	buf.WriteString("/*\n#include \"ta_func.h\"\n*/\nimport \"C\"\n\n")

	for _, d := range descs {
		writeFunc(&buf, d)
		buf.WriteByte('\n')
	}

	out, err := format.Source(buf.Bytes())
	if err != nil {
		fmt.Fprintln(os.Stderr, "warning: functions_gen.go format failed:", err)
		out = buf.Bytes()
	}
	must(os.WriteFile(path, out, 0o644))
}

func writeLookbackGen(path, version string, descs []*FuncDescriptor) {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "// Code generated by cmd/gen %s. DO NOT EDIT.\n\npackage ta\n\n", version)
	buf.WriteString(`/*
#include "ta_func.h"
*/
import "C"
`)
	buf.WriteByte('\n')

	for _, d := range descs {
		writeLookback(&buf, d)
		buf.WriteByte('\n')
	}

	out, err := format.Source(buf.Bytes())
	if err != nil {
		fmt.Fprintln(os.Stderr, "warning: lookback_gen.go format failed:", err)
		out = buf.Bytes()
	}
	must(os.WriteFile(path, out, 0o644))
}

func writeFunc(w *bytes.Buffer, d *FuncDescriptor) {
	// Doc comment.
	if d.DocComment != "" {
		for _, line := range strings.Split(d.DocComment, "\n") {
			if line == "" {
				fmt.Fprintln(w, "//")
			} else {
				fmt.Fprintf(w, "// %s\n", line)
			}
		}
	}

	// Build function signature.
	var params []string

	// Input arrays.
	for _, p := range d.Inputs {
		goType := "[]float64"
		if p.CType == "int" {
			goType = "[]int32"
		}
		params = append(params, fmt.Sprintf("%s %s", p.GoName, goType))
	}

	// Optional inputs (scalar params).
	for _, p := range d.OptIns {
		goType := "int"
		if p.CType == "double" {
			goType = "float64"
		}
		params = append(params, fmt.Sprintf("%s %s", p.GoName, goType))
	}

	// Output buffer params — always use shortened buf names to allow named returns.
	for _, p := range d.Outputs {
		goType := "[]float64"
		if p.CType == "int" {
			goType = "[]int32"
		}
		params = append(params, fmt.Sprintf("%s %s", outBufParamName(p.GoName), goType))
	}

	// Named return types — always named for documentation clarity.
	var retParts []string
	for _, p := range d.Outputs {
		goType := "[]float64"
		if p.CType == "int" {
			goType = "[]int32"
		}
		retParts = append(retParts, fmt.Sprintf("%s %s", p.GoName, goType))
	}
	retStr := " (" + strings.Join(retParts, ", ") + ")"

	fmt.Fprintf(w, "func %s(%s)%s {\n", d.GoName, strings.Join(params, ", "), retStr)

	// Guard: require at least one input element.
	if len(d.Inputs) > 0 {
		firstIn := d.Inputs[0].GoName
		fmt.Fprintf(w, "\tif len(%s) == 0 {\n\t\tpanic(&TALibError{RetCode: 2, Message: retCodeMessage(2)})\n\t}\n", firstIn)
	}

	// Lookback call.
	lookbackArgs := buildLookbackArgs(d)
	fmt.Fprintf(w, "\tlookback := int(C.TA_%s_Lookback(%s))\n", d.Name, lookbackArgs)

	// Determine length from first input.
	if len(d.Inputs) > 0 {
		fmt.Fprintf(w, "\tn := len(%s)\n", d.Inputs[0].GoName)
	}

	// Allocate output buffers.
	for _, p := range d.Outputs {
		fmt.Fprintf(w, "\t%s = reuseOrAlloc%s(%s, n)\n", p.GoName, allocSuffix(p.CType), outBufParamName(p.GoName))
		fmt.Fprintf(w, "\tfillNaN%s(%s[:lookback])\n", nanSuffix(p.CType), p.GoName)
	}

	// Call TA function.
	fmt.Fprintf(w, "\tvar begIdx, nbElem C.int\n")

	callArgs := buildCallArgs(d)
	fmt.Fprintf(w, "\trc := C.TA_%s(0, C.int(n-1),\n\t\t%s,\n\t\t&begIdx, &nbElem,\n\t\t%s)\n",
		d.Name,
		callArgs,
		buildOutPtrs(d),
	)
	fmt.Fprintf(w, "\tcheckRC(rc)\n")

	// Fill any remainder with NaN/zero defensively.
	for _, p := range d.Outputs {
		fmt.Fprintf(w, "\tfillNaN%s(%s[lookback+int(nbElem):])\n", nanSuffix(p.CType), p.GoName)
	}

	// Return.
	if len(d.Outputs) == 1 {
		fmt.Fprintf(w, "\treturn %s\n", d.Outputs[0].GoName)
	} else {
		var names []string
		for _, p := range d.Outputs {
			names = append(names, p.GoName)
		}
		fmt.Fprintf(w, "\treturn %s\n", strings.Join(names, ", "))
	}

	fmt.Fprintln(w, "}")
}

func allocSuffix(ctype string) string {
	if ctype == "int" {
		return "Int32"
	}
	return ""
}

func nanSuffix(ctype string) string {
	if ctype == "int" {
		return "Int32"
	}
	return ""
}

func buildLookbackArgs(d *FuncDescriptor) string {
	var args []string
	for _, p := range d.OptIns {
		switch p.CType {
		case "double":
			args = append(args, fmt.Sprintf("C.double(%s)", p.GoName))
		case "TA_MAType":
			args = append(args, fmt.Sprintf("C.TA_MAType(%s)", p.GoName))
		default:
			args = append(args, fmt.Sprintf("C.int(%s)", p.GoName))
		}
	}
	return strings.Join(args, ", ")
}

func buildCallArgs(d *FuncDescriptor) string {
	var args []string
	for _, p := range d.Inputs {
		if p.CType == "int" {
			args = append(args, fmt.Sprintf("inPtrInt(%s)", p.GoName))
		} else {
			args = append(args, fmt.Sprintf("inPtr(%s)", p.GoName))
		}
	}
	for _, p := range d.OptIns {
		switch p.CType {
		case "double":
			args = append(args, fmt.Sprintf("C.double(%s)", p.GoName))
		case "TA_MAType":
			args = append(args, fmt.Sprintf("C.TA_MAType(%s)", p.GoName))
		default:
			args = append(args, fmt.Sprintf("C.int(%s)", p.GoName))
		}
	}
	return strings.Join(args, ",\n\t\t")
}

func buildOutPtrs(d *FuncDescriptor) string {
	var args []string
	for _, p := range d.Outputs {
		if p.CType == "int" {
			args = append(args, fmt.Sprintf("inPtrInt(%s[lookback:])", p.GoName))
		} else {
			args = append(args, fmt.Sprintf("inPtr(%s[lookback:])", p.GoName))
		}
	}
	return strings.Join(args, ",\n\t\t")
}

func writeLookback(w *bytes.Buffer, d *FuncDescriptor) {
	// Build params from OptIns only (same as LookbackSig).
	var params []string
	for _, p := range d.OptIns {
		goType := "int"
		if p.CType == "double" {
			goType = "float64"
		} else if p.CType == "TA_MAType" {
			goType = "int"
		}
		params = append(params, fmt.Sprintf("%s %s", p.GoName, goType))
	}

	fmt.Fprintf(w, "// %sLookback returns the number of input values consumed before the first valid output.\n", d.GoName)
	fmt.Fprintf(w, "func %sLookback(%s) int {\n", d.GoName, strings.Join(params, ", "))

	cArgs := buildLookbackArgs(d)
	fmt.Fprintf(w, "\treturn int(C.TA_%s_Lookback(%s))\n", d.Name, cArgs)
	fmt.Fprintln(w, "}")
}

