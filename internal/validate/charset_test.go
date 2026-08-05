package validate

import (
	"strings"
	"testing"

	"github.com/cocosip/go-dicom/pkg/dicom/charset"
	"github.com/cocosip/go-dicom/pkg/dicom/dataset"
	"github.com/cocosip/go-dicom/pkg/dicom/element"
	"github.com/cocosip/go-dicom/pkg/dicom/tag"
	"github.com/cocosip/go-dicom/pkg/dicom/vr"
	"golang.org/x/text/encoding/unicode"
)

func TestCheckCharacterSetWarnsWhenDeclaredGB18030ContainsUTF8(t *testing.T) {
	dataset := charsetDataset("GB18030",
		element.NewStringWithEncoding(tag.PatientName, vr.PN, []string{"张三"}, unicode.UTF8),
		element.NewStringWithEncoding(tag.InstitutionName, vr.LO, []string{"示例医院"}, unicode.UTF8),
	)

	issues := CheckCharacterSet(dataset)
	if len(issues) != 1 {
		t.Fatalf("issues = %#v, want one warning", issues)
	}
	issue := issues[0]
	if issue.Source != "dicom-cli.charset" || issue.Path != "SpecificCharacterSet" || issue.Severity != Warning {
		t.Fatalf("issue = %#v", issue)
	}
	for _, value := range []string{"declared=GB18030", "recommended=ISO_IR 192", "PatientName", "InstitutionName"} {
		if !strings.Contains(issue.Message, value) {
			t.Fatalf("message = %q, want %q", issue.Message, value)
		}
	}
	for _, value := range []string{"张三", "示例医院"} {
		if strings.Contains(issue.Message, value) {
			t.Fatalf("message leaked text %q: %q", value, issue.Message)
		}
	}
}

func TestCheckCharacterSetSkipsGenuineAndAmbiguousText(t *testing.T) {
	tests := []struct {
		name     string
		charset  string
		encoding *charset.Info
		values   []string
	}{
		{
			name:     "genuine GB18030",
			charset:  "GB18030",
			encoding: mustCharsetInfo(t, "GB18030"),
			values:   []string{"张三", "示例医院"},
		},
		{
			name:     "ASCII only",
			charset:  "GB18030",
			encoding: mustCharsetInfo(t, "GB18030"),
			values:   []string{"SYNTHETIC", "Example Hospital"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataset := charsetDataset(tt.charset,
				element.NewStringWithEncoding(tag.PatientName, vr.PN, []string{tt.values[0]}, tt.encoding.Encoding),
				element.NewStringWithEncoding(tag.InstitutionName, vr.LO, []string{tt.values[1]}, tt.encoding.Encoding),
			)
			if issues := CheckCharacterSet(dataset); len(issues) != 0 {
				t.Fatalf("issues = %#v, want none", issues)
			}
		})
	}
}

func TestCheckCharacterSetReportsDecodeFailure(t *testing.T) {
	dataset := charsetDataset("GB18030",
		element.NewStringWithEncoding(tag.PatientName, vr.PN, []string{"ࠀ"}, unicode.UTF8),
	)

	issues := CheckCharacterSet(dataset)
	if len(issues) != 1 {
		t.Fatalf("issues = %#v, want one error", issues)
	}
	if issues[0].Severity != Error || !strings.Contains(issues[0].Message, "recommended=ISO_IR 192") {
		t.Fatalf("issue = %#v", issues[0])
	}
}

func TestCheckCharacterSetUsesInheritedAndItemSpecificDeclarations(t *testing.T) {
	root := charsetDataset("GB18030")
	inherited := dataset.New()
	if err := inherited.Add(element.NewStringWithEncoding(tag.PatientName, vr.PN, []string{"张三"}, unicode.UTF8)); err != nil {
		t.Fatal(err)
	}
	if err := inherited.Add(element.NewStringWithEncoding(tag.InstitutionName, vr.LO, []string{"示例医院"}, unicode.UTF8)); err != nil {
		t.Fatal(err)
	}
	local := charsetDataset("ISO_IR 192",
		element.NewStringWithEncoding(tag.PatientName, vr.PN, []string{"李四"}, unicode.UTF8),
	)
	if err := root.Add(dataset.NewSequenceWithItems(tag.ContentSequence, []*dataset.Dataset{inherited, local})); err != nil {
		t.Fatal(err)
	}

	issues := CheckCharacterSet(root)
	if len(issues) != 1 {
		t.Fatalf("issues = %#v, want one inherited-scope warning", issues)
	}
	if issues[0].Path != "SpecificCharacterSet" || !strings.Contains(issues[0].Message, "ContentSequence[0].PatientName") {
		t.Fatalf("issue = %#v", issues[0])
	}
}

func charsetDataset(declared string, elements ...element.Element) *dataset.Dataset {
	dataset := dataset.New()
	if err := dataset.Add(element.NewString(tag.SpecificCharacterSet, vr.CS, []string{declared})); err != nil {
		panic(err)
	}
	for _, value := range elements {
		if err := dataset.Add(value); err != nil {
			panic(err)
		}
	}
	return dataset
}

func mustCharsetInfo(t *testing.T, value string) *charset.Info {
	t.Helper()
	info, ok := charset.GetCharsetInfo(value)
	if !ok {
		t.Fatalf("charset.GetCharsetInfo(%q) = false", value)
	}
	return info
}
