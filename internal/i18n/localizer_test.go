package i18n

import "testing"

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
