package symbols

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

type Language string

const (
	LangGo           Language = "go"
	LangPython       Language = "python"
	LangJavaScript   Language = "javascript"
	LangTypeScript   Language = "typescript"
	LangRust         Language = "rust"
	LangJava         Language = "java"
	LangCpp          Language = "cpp"
	LangC            Language = "c"
	LangCSharp       Language = "c_sharp"
	LangRuby         Language = "ruby"
	LangPHP          Language = "php"
	LangSwift        Language = "swift"
	LangKotlin       Language = "kotlin"
	LangScala        Language = "scala"
)

type SymbolKind int

const (
	SymbolFunction SymbolKind = iota
	SymbolMethod
	SymbolStruct
	SymbolClass
	SymbolInterface
	SymbolTrait
	SymbolEnum
	SymbolTypeAlias
	SymbolConst
	SymbolVar
	SymbolField
	SymbolModule
	SymbolImport
	SymbolExport
	SymbolProtocol
	SymbolObject
)

func (k SymbolKind) String() string {
	switch k {
	case SymbolFunction:
		return "func"
	case SymbolMethod:
		return "method"
	case SymbolStruct:
		return "struct"
	case SymbolClass:
		return "class"
	case SymbolInterface:
		return "interface"
	case SymbolTrait:
		return "trait"
	case SymbolEnum:
		return "enum"
	case SymbolTypeAlias:
		return "alias"
	case SymbolConst:
		return "const"
	case SymbolVar:
		return "var"
	case SymbolField:
		return "field"
	case SymbolModule:
		return "module"
	case SymbolImport:
		return "import"
	case SymbolExport:
		return "export"
	case SymbolProtocol:
		return "protocol"
	case SymbolObject:
		return "object"
	}
	return "unknown"
}

type Symbol struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Kind       SymbolKind `json:"kind"`
	Language   Language   `json:"language"`
	FilePath   string     `json:"file_path"`
	Line       int        `json:"line"`
	Column     int        `json:"column"`
	EndLine    int        `json:"end_line"`
	Signature  string     `json:"signature"`
	DocComment string     `json:"doc_comment,omitempty"`
	IsExported bool       `json:"is_exported"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

type CallEdge struct {
	CallerID string `json:"caller_id"`
	CalleeID string `json:"callee_id"`
	Line     int    `json:"line"`
}

type ImportEdge struct {
	ImporterID string `json:"importer_id"`
	Imported   string `json:"imported"`
	Alias      string `json:"alias,omitempty"`
	Line       int    `json:"line"`
}

type SymbolGraph struct {
	mu           sync.RWMutex
	symbols      map[string]*Symbol
	callEdges    []CallEdge
	importEdges  []ImportEdge
	fileIndex    map[string][]string
	nameIndex    map[string][]string
	projectRoot  string
}

func NewSymbolGraph(root string) *SymbolGraph {
	return &SymbolGraph{
		symbols:     make(map[string]*Symbol),
		callEdges:   []CallEdge{},
		importEdges: []ImportEdge{},
		fileIndex:   make(map[string][]string),
		nameIndex:   make(map[string][]string),
		projectRoot: root,
	}
}

func (g *SymbolGraph) AddSymbol(s *Symbol) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.symbols[s.ID] = s
	g.fileIndex[s.FilePath] = append(g.fileIndex[s.FilePath], s.ID)
	g.nameIndex[s.Name] = append(g.nameIndex[s.Name], s.ID)
}

func (g *SymbolGraph) GetSymbol(id string) (*Symbol, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	s, ok := g.symbols[id]
	return s, ok
}

func (g *SymbolGraph) GetSymbolsByFile(filePath string) []*Symbol {
	g.mu.RLock()
	defer g.mu.RUnlock()
	ids := g.fileIndex[filePath]
	result := make([]*Symbol, 0, len(ids))
	for _, id := range ids {
		if s, ok := g.symbols[id]; ok {
			result = append(result, s)
		}
	}
	return result
}

func (g *SymbolGraph) Lookup(name string) []*Symbol {
	g.mu.RLock()
	defer g.mu.RUnlock()
	ids := g.nameIndex[name]
	result := make([]*Symbol, 0, len(ids))
	for _, id := range ids {
		if s, ok := g.symbols[id]; ok {
			result = append(result, s)
		}
	}
	return result
}

func (g *SymbolGraph) GetAllSymbols() []*Symbol {
	g.mu.RLock()
	defer g.mu.RUnlock()
	result := make([]*Symbol, 0, len(g.symbols))
	for _, s := range g.symbols {
		result = append(result, s)
	}
	return result
}

func (g *SymbolGraph) AddCallEdge(edge CallEdge) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.callEdges = append(g.callEdges, edge)
}

func (g *SymbolGraph) AddImportEdge(edge ImportEdge) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.importEdges = append(g.importEdges, edge)
}

func (g *SymbolGraph) GetCallers(calleeID string) []string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	var callers []string
	for _, e := range g.callEdges {
		if e.CalleeID == calleeID {
			callers = append(callers, e.CallerID)
		}
	}
	return callers
}

func (g *SymbolGraph) GetCallees(callerID string) []string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	var callees []string
	for _, e := range g.callEdges {
		if e.CallerID == callerID {
			callees = append(callees, e.CalleeID)
		}
	}
	return callees
}

func (g *SymbolGraph) GetImports(filePath string) []ImportEdge {
	g.mu.RLock()
	defer g.mu.RUnlock()
	var result []ImportEdge
	for _, e := range g.importEdges {
		if strings.HasPrefix(e.ImporterID, filePath) {
			result = append(result, e)
		}
	}
	return result
}

func (g *SymbolGraph) Count() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.symbols)
}

type Indexer struct {
	graph     *SymbolGraph
	regexes   map[Language][]symbolPattern
	importers map[Language][]importPattern
}

type symbolPattern struct {
	kind   SymbolKind
	regex  *regexp.Regexp
	nameGrp int
}

type importPattern struct {
	regex *regexp.Regexp
}

func NewIndexer(root string) *Indexer {
	idx := &Indexer{graph: NewSymbolGraph(root)}
	idx.regexes = buildSymbolRegexes()
	idx.importers = buildImportRegexes()
	return idx
}

func (idx *Indexer) Graph() *SymbolGraph { return idx.graph }

func (idx *Indexer) IndexFile(filePath string) error {
	lang := detectLanguage(filePath)
	if lang == "" {
		return nil
	}
	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	idx.parseFile(lang, filePath, string(content))
	return nil
}

func (idx *Indexer) IndexDirectory(root string, skip func(path string) bool) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			name := info.Name()
			if skip != nil && skip(path) {
				return filepath.SkipDir
			}
			if name == ".git" || name == "node_modules" || name == "vendor" ||
				name == ".cache" || name == "dist" || name == "build" || name == "target" {
				return filepath.SkipDir
			}
			return nil
		}
		lang := detectLanguage(path)
		if lang == "" {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		idx.parseFile(lang, path, string(content))
		return nil
	})
}

func (idx *Indexer) parseFile(lang Language, filePath string, content string) {
	patterns := idx.regexes[lang]
	if len(patterns) == 0 {
		return
	}

	lines := strings.Split(content, "\n")

	for _, pat := range patterns {
		matches := pat.regex.FindAllStringSubmatchIndex(content, -1)
		for _, m := range matches {
			nameStart := m[pat.nameGrp*2]
			nameEnd := m[pat.nameGrp*2+1]
			if nameStart < 0 || nameEnd < 0 {
				continue
			}
			name := content[nameStart:nameEnd]
			if name == "" {
				continue
			}
			line, col := offsetToLineCol(content, m[0])
			endLine, _ := offsetToLineCol(content, m[1])
			id := fmt.Sprintf("%s:%d:%d", filePath, line, col)
			sigStart := m[0]
			sigEnd := m[1]
			if sigEnd-sigStart < 400 {
				if nl := strings.IndexByte(content[sigEnd:], '\n'); nl >= 0 {
					sigEnd = sigEnd + nl
				} else {
					sigEnd = len(content)
				}
			}
			if sigEnd-sigStart > 400 {
				sigEnd = sigStart + 400
			}
			sig := strings.Join(strings.Fields(content[sigStart:sigEnd]), " ")

			s := &Symbol{
				ID:        id,
				Name:      name,
				Kind:      pat.kind,
				Language:  lang,
				FilePath:  filePath,
				Line:      line,
				Column:    col,
				EndLine:   endLine,
				Signature: sig,
				IsExported: isExportedName(lang, name),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}

			// Attach preceding doc comment.
			if line > 1 {
				s.DocComment = extractDocComment(lang, lines, line)
			}

			idx.graph.AddSymbol(s)

			// Call graph: functions referencing other symbols by name.
			if pat.kind == SymbolFunction || pat.kind == SymbolMethod {
				idx.extractCalls(lang, content, lines, s.ID, line)
			}
		}
	}

	// Imports.
	if importers := idx.importers[lang]; len(importers) > 0 {
		for _, imp := range importers {
			matches := imp.regex.FindAllStringSubmatch(content, -1)
			for _, m := range matches {
				if len(m) < 2 {
					continue
				}
				line, _ := offsetToLineCol(content, strings.Index(content, m[0]))
				idx.graph.AddImportEdge(ImportEdge{
					ImporterID: filePath,
					Imported:   strings.TrimSpace(m[1]),
					Line:       line,
				})
			}
		}
	}
}

func (idx *Indexer) extractCalls(lang Language, content string, lines []string, callerID string, fnLine int) {
	callers := idx.callerRegexes(lang)
	// Only scan a window around the function body (first 100 lines) for speed.
	limit := fnLine + 100
	if limit > len(lines) {
		limit = len(lines)
	}
	body := strings.Join(lines[fnLine-1:limit], "\n")
	for _, re := range callers {
		for _, m := range re.FindAllStringSubmatchIndex(body, -1) {
			if len(m) < 4 {
				continue
			}
			callee := body[m[2]:m[3]]
			if callee == "" {
				continue
			}
			cLine, _ := offsetToLineCol(body, m[0])
			// Resolve to a symbol if it exists in the graph.
			resolved := idx.graph.Lookup(callee)
			if len(resolved) > 0 {
				idx.graph.AddCallEdge(CallEdge{
					CallerID: callerID,
					CalleeID: resolved[0].ID,
					Line:     cLine + fnLine - 1,
				})
			}
		}
	}
}

func (idx *Indexer) callerRegexes(lang Language) []*regexp.Regexp {
	switch lang {
	case LangGo:
		return []*regexp.Regexp{regexp.MustCompile(`\b([a-zA-Z_][a-zA-Z0-9_]*)\s*\(`)}
	case LangPython:
		return []*regexp.Regexp{regexp.MustCompile(`\b([a-zA-Z_][a-zA-Z0-9_]*)\s*\(`)}
	case LangJavaScript, LangTypeScript:
		return []*regexp.Regexp{regexp.MustCompile(`\b([a-zA-Z_$][a-zA-Z0-9_$]*)\s*\(`)}
	case LangRust:
		return []*regexp.Regexp{regexp.MustCompile(`\b([a-zA-Z_][a-zA-Z0-9_]*)\s*\(`)}
	case LangJava:
		return []*regexp.Regexp{regexp.MustCompile(`\b([a-zA-Z_][a-zA-Z0-9_]*)\s*\(`)}
	case LangCpp, LangC, LangCSharp:
		return []*regexp.Regexp{regexp.MustCompile(`\b([a-zA-Z_][a-zA-Z0-9_]*)\s*\(`)}
	default:
		return []*regexp.Regexp{regexp.MustCompile(`\b([a-zA-Z_][a-zA-Z0-9_]*)\s*\(`)}
	}
}

func detectLanguage(path string) Language {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return LangGo
	case ".py", ".pyw":
		return LangPython
	case ".js", ".jsx", ".mjs", ".cjs":
		return LangJavaScript
	case ".ts", ".tsx":
		return LangTypeScript
	case ".rs":
		return LangRust
	case ".java":
		return LangJava
	case ".cpp", ".cc", ".cxx", ".hpp", ".hh", ".h":
		return LangCpp
	case ".c":
		return LangC
	case ".cs":
		return LangCSharp
	case ".rb":
		return LangRuby
	case ".php":
		return LangPHP
	case ".swift":
		return LangSwift
	case ".kt", ".kts":
		return LangKotlin
	case ".scala":
		return LangScala
	default:
		return ""
	}
}

func offsetToLineCol(s string, off int) (int, int) {
	if off < 0 || off > len(s) {
		return 1, 0
	}
	line := 1
	col := 0
	for i := 0; i < off; i++ {
		if s[i] == '\n' {
			line++
			col = 0
		} else {
			col++
		}
	}
	return line, col
}

func extractDocComment(lang Language, lines []string, fnLine int) string {
	var doc []string
	for i := fnLine - 2; i >= 0; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trimmed, "//") {
			doc = append([]string{strings.TrimSpace(strings.TrimPrefix(trimmed, "//"))}, doc...)
			continue
		}
		if strings.HasPrefix(trimmed, "#") && lang == LangPython {
			doc = append([]string{strings.TrimSpace(strings.TrimPrefix(trimmed, "#"))}, doc...)
			continue
		}
		if strings.HasPrefix(trimmed, "///") || strings.HasPrefix(trimmed, "/*") {
			doc = append([]string{strings.Trim(trimmed, "/ *")}, doc...)
			continue
		}
		if strings.HasPrefix(trimmed, "\"\"\"") && lang == LangPython {
			doc = append([]string{strings.Trim(trimmed, "\"")}, doc...)
			continue
		}
		break
	}
	return strings.Join(doc, " ")
}

func isExportedName(lang Language, name string) bool {
	if name == "" {
		return false
	}
	switch lang {
	case LangGo:
		return strings.ToUpper(name[:1]) == name[:1]
	case LangPython:
		return !strings.HasPrefix(name, "_")
	case LangJavaScript, LangTypeScript:
		return !strings.HasPrefix(name, "_")
	default:
		return true
	}
}

func buildSymbolRegexes() map[Language][]symbolPattern {
	rg := func(kind SymbolKind, re string) symbolPattern {
		return symbolPattern{kind: kind, regex: regexp.MustCompile(re), nameGrp: 1}
	}
	return map[Language][]symbolPattern{
		LangGo: {
			rg(SymbolFunction, `(?m)^func\s+(?:\([^)]*\)\s*)?([a-zA-Z_][a-zA-Z0-9_]*)\s*\(`),
			rg(SymbolStruct, `(?m)^type\s+([a-zA-Z_][a-zA-Z0-9_]*)\s+struct\s*\{`),
			rg(SymbolInterface, `(?m)^type\s+([a-zA-Z_][a-zA-Z0-9_]*)\s+interface\s*\{`),
			rg(SymbolTypeAlias, `(?m)^type\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*=\s*`),
			rg(SymbolConst, `(?m)^const\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*=`),
			rg(SymbolVar, `(?m)^var\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*=`),
		},
		LangPython: {
			rg(SymbolFunction, `(?m)^(?:async\s+)?def\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*\(`),
			rg(SymbolClass, `(?m)^class\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*[:(]`),
			rg(SymbolConst, `(?m)^([A-Z][A-Z0-9_]{2,})\s*=`),
		},
		LangJavaScript: {
			rg(SymbolFunction, `(?m)^(?:async\s+)?function\s+([a-zA-Z_$][a-zA-Z0-9_$]*)\s*\(`),
			rg(SymbolClass, `(?m)^class\s+([a-zA-Z_$][a-zA-Z0-9_$]*)\s*\{`),
			rg(SymbolConst, `(?m)^const\s+([a-zA-Z_$][a-zA-Z0-9_$]*)\s*=`),
			rg(SymbolVar, `(?m)^let\s+([a-zA-Z_$][a-zA-Z0-9_$]*)\s*=`),
			rg(SymbolExport, `(?m)^export\s+(?:default\s+)?(?:function|class)\s+([a-zA-Z_$][a-zA-Z0-9_$]*)`),
		},
		LangTypeScript: {
			rg(SymbolFunction, `(?m)^(?:async\s+)?function\s+([a-zA-Z_$][a-zA-Z0-9_$]*)\s*\(`),
			rg(SymbolClass, `(?m)^class\s+([a-zA-Z_$][a-zA-Z0-9_$]*)\s*(?:extends|implements|\{)`),
			rg(SymbolInterface, `(?m)^interface\s+([a-zA-Z_$][a-zA-Z0-9_$]*)\s*(?:extends)?\s*\{`),
			rg(SymbolTypeAlias, `(?m)^type\s+([a-zA-Z_$][a-zA-Z0-9_$]*)\s*=`),
			rg(SymbolEnum, `(?m)^enum\s+([a-zA-Z_$][a-zA-Z0-9_$]*)\s*\{`),
			rg(SymbolConst, `(?m)^(?:export\s+)?const\s+([a-zA-Z_$][a-zA-Z0-9_$]*)\s*=`),
		},
		LangRust: {
			rg(SymbolFunction, `(?m)^(?:(?:pub|async|unsafe|extern)\s+)*(?:fn\s+)([a-zA-Z_][a-zA-Z0-9_]*)\s*\(`),
			rg(SymbolStruct, `(?m)^(?:pub\s+)?struct\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*\{`),
			rg(SymbolEnum, `(?m)^(?:pub\s+)?enum\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*\{`),
			rg(SymbolTrait, `(?m)^(?:pub\s+)?trait\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*\{`),
			rg(SymbolTypeAlias, `(?m)^(?:pub\s+)?type\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*=`),
		},
		LangJava: {
			rg(SymbolClass, `(?m)^(?:public|private|protected|final|abstract|static)*\s*class\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*\{`),
			rg(SymbolInterface, `(?m)^(?:public\s+)?interface\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*\{`),
			rg(SymbolEnum, `(?m)^(?:public\s+)?enum\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*\{`),
			rg(SymbolMethod, `(?m)^\s*(?:public|private|protected)\s+(?:static\s+)?[\w<>\[\],\s]+?\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*\([^)]*\)\s*\{`),
		},
		LangCpp: {
			rg(SymbolFunction, `(?m)^\s*(?:inline|static|extern|template\s*<[^>]+>|virtual|constexpr|noexcept|override|final)*\s*[\w:<>\[\],\s*&]+?\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*\([^;]*\)\s*\{`),
			rg(SymbolClass, `(?m)^\s*class\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*[:\{]`),
			rg(SymbolStruct, `(?m)^\s*struct\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*[:\{]`),
			rg(SymbolEnum, `(?m)^\s*enum\s+(?:class\s+)?([a-zA-Z_][a-zA-Z0-9_]*)\s*\{`),
		},
		LangC: {
			rg(SymbolFunction, `(?m)^\s*(?:static|inline|extern|const)?\s*[\w\*\[\]\s]+?\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*\([^;]*\)\s*\{`),
			rg(SymbolStruct, `(?m)^\s*typedef\s+struct\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*\{`),
			rg(SymbolTypeAlias, `(?m)^\s*typedef\s+[\w\*]+\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*;`),
		},
		LangCSharp: {
			rg(SymbolClass, `(?m)^\s*(?:public|internal|private|sealed|abstract|static|partial)*\s*class\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*[:\{<]`),
			rg(SymbolInterface, `(?m)^\s*(?:public|internal)\s*interface\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*[:\{<]`),
			rg(SymbolStruct, `(?m)^\s*(?:public|internal)\s*struct\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*[:\{<]`),
			rg(SymbolEnum, `(?m)^\s*(?:public|internal)\s*enum\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*\{`),
			rg(SymbolMethod, `(?m)^\s*(?:public|private|protected|internal)\s+(?:static\s+)?[\w<>\[\],\s?]+?\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*\([^)]*\)\s*\{`),
		},
		LangRuby: {
			rg(SymbolMethod, `(?m)^\s*def\s+([a-zA-Z_][a-zA-Z0-9_]*[!?]?)\s*[\($]`),
			rg(SymbolClass, `(?m)^\s*class\s+([A-Z][a-zA-Z0-9_:]*)\s*[<$]`),
			rg(SymbolModule, `(?m)^\s*module\s+([A-Z][a-zA-Z0-9_:]*)\s*$`),
			rg(SymbolConst, `(?m)^\s*([A-Z][A-Z0-9_]{2,})\s*=`),
		},
		LangPHP: {
			rg(SymbolFunction, `(?m)^\s*function\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*\(`),
			rg(SymbolClass, `(?m)^\s*(?:abstract\s+|final\s+)?class\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*[\{:a]`),
			rg(SymbolInterface, `(?m)^\s*interface\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*[\{:a]`),
			rg(SymbolTrait, `(?m)^\s*trait\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*\{`),
			rg(SymbolMethod, `(?m)^\s*(?:public|private|protected|static|final|abstract)*\s*function\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*\(`),
		},
		LangSwift: {
			rg(SymbolFunction, `(?m)^\s*(?:public|private|internal|fileprivate)?\s*(?:static|class|override)?\s*func\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*\(`),
			rg(SymbolClass, `(?m)^\s*(?:public|private|internal|final)?\s*class\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*[:\{]`),
			rg(SymbolStruct, `(?m)^\s*(?:public|private|internal)?\s*struct\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*[:\{]`),
			rg(SymbolEnum, `(?m)^\s*(?:public|private|internal)?\s*enum\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*[:\{]`),
			rg(SymbolProtocol, `(?m)^\s*(?:public|private|internal)?\s*protocol\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*[:\{]`),
		},
		LangKotlin: {
			rg(SymbolFunction, `(?m)^\s*(?:public|private|internal|protected)?\s*(?:suspend\s+)?fun\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*\(`),
			rg(SymbolClass, `(?m)^\s*(?:public|private|internal|data|sealed|abstract|open|final)?\s*class\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*[\(:\{<]`),
			rg(SymbolInterface, `(?m)^\s*(?:public|private|internal)?\s*interface\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*[:\{<]`),
			rg(SymbolEnum, `(?m)^\s*(?:public|private|internal)?\s*enum\s+class\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*[:\{<]`),
		},
		LangScala: {
			rg(SymbolClass, `(?m)^\s*(?:case\s+)?class\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*[\(:\{<]`),
			rg(SymbolObject, `(?m)^\s*(?:case\s+)?object\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*[\(:\{<]`),
			rg(SymbolTrait, `(?m)^\s*trait\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*[\(:\{<]`),
			rg(SymbolFunction, `(?m)^\s*def\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*[\[\(:]`),
		},
	}
}

func buildImportRegexes() map[Language][]importPattern {
	re := func(s string) importPattern { return importPattern{regex: regexp.MustCompile(s)} }
	return map[Language][]importPattern{
		LangGo:           {re(`(?m)^\s*import\s+"([^"]+)"`), re(`(?m)^\s*(\w+)\s+"([^"]+)"`), re(`(?m)^\s*import\s*\(\s*\n([^)]*)\)`)},
		LangPython:       {re(`(?m)^\s*import\s+([a-zA-Z0-9_.]+)`), re(`(?m)^\s*from\s+([a-zA-Z0-9_.]+)\s+import\s+`)},
		LangJavaScript:   {re(`(?m)^\s*import\s+[^;]*?from\s+['"]([^'"]+)['"]`), re(`(?m)^\s*import\s+['"]([^'"]+)['"]`), re(`(?m)^\s*require\s*\(\s*['"]([^'"]+)['"]`)},
		LangTypeScript:   {re(`(?m)^\s*import\s+[^;]*?from\s+['"]([^'"]+)['"]`), re(`(?m)^\s*import\s+['"]([^'"]+)['"]`), re(`(?m)^\s*require\s*\(\s*['"]([^'"]+)['"]`)},
		LangRust:         {re(`(?m)^\s*use\s+([a-zA-Z0-9_:]+)`), re(`(?m)^\s*extern\s+crate\s+([a-zA-Z0-9_]+)`)},
		LangJava:         {re(`(?m)^\s*import\s+(?:static\s+)?([a-zA-Z0-9_.]+);`)},
		LangCpp:          {re(`(?m)^\s*#include\s*[<"]([^>"]+)[>"]`)},
		LangC:            {re(`(?m)^\s*#include\s*[<"]([^>"]+)[>"]`)},
		LangCSharp:       {re(`(?m)^\s*using\s+([a-zA-Z0-9_.]+);`)},
		LangRuby:         {re(`(?m)^\s*require(?:_relative)?\s+['"]([^'"]+)['"]`)},
		LangPHP:          {re(`(?m)^\s*use\s+([a-zA-Z0-9_\\]+)`), re(`(?m)^\s*require(?:_once)?\s*\(\s*['"]([^'"]+)['"]`)},
		LangSwift:        {re(`(?m)^\s*import\s+([a-zA-Z0-9_.]+)`)},
		LangKotlin:       {re(`(?m)^\s*import\s+([a-zA-Z0-9_.]+)`)},
		LangScala:        {re(`(?m)^\s*import\s+([a-zA-Z0-9_.]+)`)},
	}
}

func (idx *Indexer) SaveIndex(path string) error {
	data, err := idx.graph.Serialize()
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func (idx *Indexer) LoadIndex(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return idx.graph.Deserialize(data)
}

func (g *SymbolGraph) Serialize() ([]byte, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	type snapshot struct {
		Symbols     []*Symbol    `json:"symbols"`
		CallEdges   []CallEdge   `json:"call_edges"`
		ImportEdges []ImportEdge `json:"import_edges"`
	}
	snap := snapshot{
		Symbols:     make([]*Symbol, 0, len(g.symbols)),
		CallEdges:   g.callEdges,
		ImportEdges: g.importEdges,
	}
	for _, s := range g.symbols {
		snap.Symbols = append(snap.Symbols, s)
	}
	return json.Marshal(snap)
}

func (g *SymbolGraph) Deserialize(data []byte) error {
	var snap struct {
		Symbols     []*Symbol    `json:"symbols"`
		CallEdges   []CallEdge   `json:"call_edges"`
		ImportEdges []ImportEdge `json:"import_edges"`
	}
	if err := json.Unmarshal(data, &snap); err != nil {
		return err
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.symbols = make(map[string]*Symbol)
	g.fileIndex = make(map[string][]string)
	g.nameIndex = make(map[string][]string)
	for _, s := range snap.Symbols {
		g.symbols[s.ID] = s
		g.fileIndex[s.FilePath] = append(g.fileIndex[s.FilePath], s.ID)
		g.nameIndex[s.Name] = append(g.nameIndex[s.Name], s.ID)
	}
	g.callEdges = snap.CallEdges
	g.importEdges = snap.ImportEdges
	return nil
}

// TopSymbols returns the most referenced symbols across the repo (fan-in).
func (g *SymbolGraph) TopSymbols(limit int) []*Symbol {
	g.mu.RLock()
	defer g.mu.RUnlock()
	fanIn := make(map[string]int)
	for _, e := range g.callEdges {
		fanIn[e.CalleeID]++
	}
	type pair struct {
		id  string
		fan int
	}
	var pairs []pair
	for id, f := range fanIn {
		pairs = append(pairs, pair{id, f})
	}
	for i := 0; i < len(pairs); i++ {
		for j := i + 1; j < len(pairs); j++ {
			if pairs[j].fan > pairs[i].fan {
				pairs[i], pairs[j] = pairs[j], pairs[i]
			}
		}
	}
	if len(pairs) > limit {
		pairs = pairs[:limit]
	}
	result := make([]*Symbol, 0, len(pairs))
	for _, p := range pairs {
		if s, ok := g.symbols[p.id]; ok {
			result = append(result, s)
		}
	}
	return result
}

// FindSymbolsInRange finds symbols whose file matches and line range overlaps.
func (g *SymbolGraph) FindSymbolsInRange(filePath string, line int) []*Symbol {
	g.mu.RLock()
	defer g.mu.RUnlock()
	var result []*Symbol
	for _, id := range g.fileIndex[filePath] {
		s := g.symbols[id]
		if s != nil && line >= s.Line && line <= s.EndLine {
			result = append(result, s)
		}
	}
	return result
}
