package validate

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/cocosip/go-dicom/pkg/dicom/charset"
	"github.com/cocosip/go-dicom/pkg/dicom/dataset"
	"github.com/cocosip/go-dicom/pkg/dicom/dict"
	"github.com/cocosip/go-dicom/pkg/dicom/element"
	"github.com/cocosip/go-dicom/pkg/dicom/tag"
	"golang.org/x/text/encoding"
)

var charsetCandidates = []string{"ISO_IR 192", "GB18030", "GBK"}

type charsetCandidate struct {
	name     string
	encoding encoding.Encoding
}

type charsetField struct {
	path string
	raw  []byte
}

type charsetScope struct {
	declaration     string
	declarationPath string
	fields          []charsetField
}

type charsetScore struct {
	valid            bool
	score            int
	nonASCIIFields   int
	nonASCIIFieldRef []string
}

// CheckCharacterSet reports raw textual values that do not agree with their
// active Specific Character Set declaration. It never includes decoded values
// or raw bytes in an issue because validation reports can contain PHI.
func CheckCharacterSet(root *dataset.Dataset) []Issue {
	scopes := make(map[string]*charsetScope)
	collectCharsetScopes(root, "", "", "", scopes)

	paths := make([]string, 0, len(scopes))
	for path := range scopes {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	issues := make([]Issue, 0)
	for _, path := range paths {
		if issue, ok := analyzeCharsetScope(scopes[path]); ok {
			issues = append(issues, issue)
		}
	}
	return issues
}

func collectCharsetScopes(ds *dataset.Dataset, declaration, declarationPath, prefix string, scopes map[string]*charsetScope) {
	if ds == nil {
		return
	}
	activeDeclaration, activePath := declaration, declarationPath
	if elem, ok := ds.Get(tag.SpecificCharacterSet); ok {
		if raw := trimTextPadding(elem.Buffer().Data()); len(raw) > 0 {
			activeDeclaration = string(raw)
			activePath = joinCharsetPath(prefix, "SpecificCharacterSet")
		}
	}
	if activeDeclaration != "" {
		if _, ok := scopes[activePath]; !ok {
			scopes[activePath] = &charsetScope{declaration: activeDeclaration, declarationPath: activePath}
		}
	}

	for _, elem := range ds.Elements() {
		if elem.Tag().Equals(tag.SpecificCharacterSet) {
			continue
		}
		if sequence, ok := elem.(*dataset.Sequence); ok {
			sequencePath := joinCharsetPath(prefix, tagKeyword(sequence.Tag()))
			for index, item := range sequence.GetItems() {
				collectCharsetScopes(item, activeDeclaration, activePath, fmt.Sprintf("%s[%d]", sequencePath, index), scopes)
			}
			continue
		}
		if activeDeclaration == "" || !isTextVR(elem) {
			continue
		}
		raw := trimTextPadding(elem.Buffer().Data())
		if len(raw) == 0 {
			continue
		}
		scopes[activePath].fields = append(scopes[activePath].fields, charsetField{
			path: joinCharsetPath(prefix, tagKeyword(elem.Tag())),
			raw:  append([]byte(nil), raw...),
		})
	}
}

func analyzeCharsetScope(scope *charsetScope) (Issue, bool) {
	if scope == nil || len(scope.fields) == 0 {
		return Issue{}, false
	}
	candidates, declaredIndex, ok := candidatesFor(scope.declaration)
	if !ok {
		return Issue{}, false
	}

	scores := make([]charsetScore, len(candidates))
	for index, candidate := range candidates {
		scores[index].valid = true
		for _, field := range scope.fields {
			text, valid := decodeExactly(field.raw, candidate)
			if !valid {
				scores[index].valid = false
				continue
			}
			score, nonASCII := scoreText(text)
			scores[index].score += score
			if nonASCII {
				scores[index].nonASCIIFields++
				scores[index].nonASCIIFieldRef = append(scores[index].nonASCIIFieldRef, field.path)
			}
		}
	}

	bestIndex := bestCharsetScore(scores)
	if bestIndex < 0 || bestIndex == declaredIndex {
		return Issue{}, false
	}
	declared, best := scores[declaredIndex], scores[bestIndex]
	if !best.valid || best.nonASCIIFields == 0 {
		return Issue{}, false
	}
	if !declared.valid {
		bestIndex = firstValidCharsetScore(scores, declaredIndex)
		if bestIndex < 0 {
			return Issue{}, false
		}
		best = scores[bestIndex]
		return charsetIssue(scope, candidates[bestIndex].name, Error, "confirmed", best.nonASCIIFieldRef), true
	}
	if best.nonASCIIFields < 2 || best.score <= declared.score {
		return Issue{}, false
	}
	return charsetIssue(scope, candidates[bestIndex].name, Warning, "high", best.nonASCIIFieldRef), true
}

func candidatesFor(declaration string) ([]charsetCandidate, int, bool) {
	names := append([]string{declaration}, charsetCandidates...)
	candidates := make([]charsetCandidate, 0, len(names))
	declaredIndex := -1
	for _, name := range names {
		if containsCharsetCandidate(candidates, name) {
			continue
		}
		info, known := charset.GetCharsetInfo(name)
		if !known {
			if name == declaration {
				return nil, -1, false
			}
			continue
		}
		if name == declaration {
			declaredIndex = len(candidates)
		}
		candidates = append(candidates, charsetCandidate{name: name, encoding: info.Encoding})
	}
	return candidates, declaredIndex, declaredIndex >= 0
}

func containsCharsetCandidate(candidates []charsetCandidate, name string) bool {
	for _, candidate := range candidates {
		if candidate.name == name {
			return true
		}
	}
	return false
}

func decodeExactly(raw []byte, candidate charsetCandidate) (string, bool) {
	if candidate.name == "ISO_IR 192" && !utf8.Valid(raw) {
		return "", false
	}
	decoded, err := candidate.encoding.NewDecoder().Bytes(raw)
	if err != nil || bytes.ContainsRune(decoded, unicode.ReplacementChar) {
		return "", false
	}
	encoded, err := candidate.encoding.NewEncoder().Bytes(decoded)
	if err != nil || !bytes.Equal(encoded, raw) {
		return "", false
	}
	return string(decoded), true
}

func scoreText(text string) (int, bool) {
	score, nonASCII := 0, false
	for _, value := range text {
		switch {
		case value == unicode.ReplacementChar || unicode.IsControl(value) && value != '\n' && value != '\r' && value != '\t':
			return -1000, false
		case unicode.Is(unicode.Han, value):
			score += 4
			nonASCII = true
		case value > unicode.MaxASCII:
			nonASCII = true
		case unicode.IsLetter(value) || unicode.IsDigit(value):
			score++
		}
	}
	for _, marker := range []string{"锟斤拷", "Ã", "Â", "寮犱", "绀轰", "缁撴", "妫€"} {
		if strings.Contains(text, marker) {
			score -= 20
		}
	}
	return score, nonASCII
}

func bestCharsetScore(scores []charsetScore) int {
	bestIndex := -1
	for index, score := range scores {
		if !score.valid {
			continue
		}
		if bestIndex < 0 || score.score > scores[bestIndex].score {
			bestIndex = index
		}
	}
	return bestIndex
}

func firstValidCharsetScore(scores []charsetScore, declaredIndex int) int {
	for index, score := range scores {
		if index != declaredIndex && score.valid && score.nonASCIIFields > 0 {
			return index
		}
	}
	return -1
}

func charsetIssue(scope *charsetScope, recommended string, severity Severity, confidence string, evidence []string) Issue {
	return Issue{
		Source:   "dicom-cli.charset",
		Path:     scope.declarationPath,
		Severity: severity,
		Message: fmt.Sprintf(
			"declared=%s recommended=%s confidence=%s evidence=%s",
			scope.declaration,
			recommended,
			confidence,
			strings.Join(evidence, ","),
		),
	}
}

func isTextVR(elem element.Element) bool {
	switch elem.ValueRepresentation().Code() {
	case "PN", "LO", "SH", "ST", "LT", "UC", "UT":
		return true
	default:
		return false
	}
}

func trimTextPadding(raw []byte) []byte {
	return bytes.TrimRight(raw, "\x00 ")
}

func joinCharsetPath(prefix, value string) string {
	if prefix == "" {
		return value
	}
	return prefix + "." + value
}

func tagKeyword(value *tag.Tag) string {
	if entry := dict.Default().Lookup(value); entry != nil {
		return entry.Keyword()
	}
	return value.Format("g")
}
