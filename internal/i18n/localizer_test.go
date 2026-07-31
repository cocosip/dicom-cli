package i18n

import (
	"strings"
	"testing"
)

func TestLocalizerRendersConfiguredLanguage(t *testing.T) {
	tests := []struct {
		name     string
		language string
		want     string
	}{
		{name: "english", language: English, want: "Configuration is valid"},
		{name: "simplified chinese", language: ChineseSimplified, want: "配置有效"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := New(tt.language).Text(ConfigValid); got != tt.want {
				t.Fatalf("Text(ConfigValid) = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLocalizerFormatsArguments(t *testing.T) {
	if got := New(ChineseSimplified).Text(EchoSucceeded, "pacs.example.test", 11112); got != "C-ECHO 成功：pacs.example.test:11112" {
		t.Fatalf("Text(EchoSucceeded) = %q", got)
	}
}

func TestLocalizerFallsBackToEnglish(t *testing.T) {
	if got := New("unexpected").Text(ConfigValid); got != "Configuration is valid" {
		t.Fatalf("Text(ConfigValid) = %q, want English fallback", got)
	}
}

func TestLocalizerFormatsBatchSummary(t *testing.T) {
	if got := New(ChineseSimplified).BatchSummary(4, 3, 1, 2); got != "已扫描=4 已处理=3 已跳过=1 已失败=2" {
		t.Fatalf("BatchSummary() = %q", got)
	}
}

func TestLocalizerReturnsCommandCatalogEntry(t *testing.T) {
	if got := New(English).Command("config").Short; got != "Manage runtime configuration" {
		t.Fatalf("English config short = %q", got)
	}
	if got := New(ChineseSimplified).Command("config").Long; got != "创建、校验和维护 DIMSE 命令使用的运行配置。配置发现只选择一个文件，不会合并多个配置文件。" {
		t.Fatalf("Chinese config long = %q", got)
	}
}

func TestChineseCommandCatalogCoversEveryEnglishCommand(t *testing.T) {
	localizer := New(ChineseSimplified)
	for path, english := range englishCommands {
		name := strings.TrimPrefix(path, "dicom-cli ")
		chinese := localizer.Command(name)
		if chinese.Short == "" || chinese.Long == "" {
			t.Fatalf("Chinese command %q = %+v, want non-empty text", name, chinese)
		}
		if english.Short == "" || english.Long == "" {
			t.Fatalf("English command %q = %+v, want non-empty text", name, english)
		}
	}
}
