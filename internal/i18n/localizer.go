// Package i18n renders the human-readable messages emitted by dicom-cli.
package i18n

import (
	"fmt"
	"strings"
)

const (
	English           = "en"
	ChineseSimplified = "zh-CN"
)

// Key identifies a localized message.
type Key string

// CommandText contains the human-readable text shown for one command.
// Command names, positional arguments, flags, and examples remain stable CLI
// contracts and deliberately do not belong in the translated text.
type CommandText struct {
	Short string
	Long  string
}

const (
	ConfigValid           Key = "config.valid"
	ValidationValid       Key = "validation.valid"
	EchoSucceeded         Key = "echo.succeeded"
	RootShort             Key = "root.short"
	RootLong              Key = "root.long"
	InspectShort          Key = "inspect.short"
	InspectLong           Key = "inspect.long"
	HelpUsage             Key = "help.usage"
	HelpAliases           Key = "help.aliases"
	HelpExamples          Key = "help.examples"
	HelpAvailableCommands Key = "help.available_commands"
	HelpFlags             Key = "help.flags"
	HelpGlobalFlags       Key = "help.global_flags"
	HelpAdditionalTopics  Key = "help.additional_topics"
	HelpMoreInformation   Key = "help.more_information"
	FlagHelp              Key = "flag.help"
	FlagConfig            Key = "flag.config"
	FlagRules             Key = "flag.rules"
	FlagVerbose           Key = "flag.verbose"
	FlagQuiet             Key = "flag.quiet"
	FlagLogFormat         Key = "flag.log_format"
	FlagInspectAll        Key = "flag.inspect.all"
	FlagInspectTag        Key = "flag.inspect.tag"
	FlagInspectProfile    Key = "flag.inspect.profile"
	FlagJSON              Key = "flag.json"
	FlagReportOutput      Key = "flag.report_output"
)

// Localizer selects one immutable message catalog for a command invocation.
type Localizer struct {
	language string
	messages map[Key]string
}

// New returns the selected catalog. Unknown languages use English so callers
// can report invalid configuration deterministically.
func New(language string) Localizer {
	if language == ChineseSimplified {
		return Localizer{language: ChineseSimplified, messages: chineseSimplified}
	}
	return Localizer{language: English, messages: english}
}

// IsChineseSimplified reports whether the selected catalog is Simplified Chinese.
func (localizer Localizer) IsChineseSimplified() bool {
	return localizer.language == ChineseSimplified
}

// Command returns the catalog entry for a command relative to dicom-cli, such
// as "config" or "config target add". Missing entries are intentionally empty
// so new commands cannot silently retain an English description in Chinese help.
func (localizer Localizer) Command(name string) CommandText {
	path := "dicom-cli " + name
	if localizer.IsChineseSimplified() {
		return CommandText{Short: chineseCommandShort[path], Long: chineseCommandLong[path]}
	}
	return englishCommands[path]
}

// FlagUsage returns the localized usage description for common CLI flags.
func (localizer Localizer) FlagUsage(name, fallback string) string {
	if !localizer.IsChineseSimplified() {
		return fallback
	}
	if translated, ok := chineseFlagUsage[name]; ok {
		return translated
	}
	return fallback
}

// ReplaceInspectLabels localizes only the fixed labels of the text report.
// DICOM values, Tag paths, UIDs, and JSON are intentionally left unchanged.
func (localizer Localizer) ReplaceInspectLabels(value string) string {
	if !localizer.IsChineseSimplified() {
		return value
	}
	replacements := make([]string, 0, len(chineseInspectLabels)*2)
	for english, chinese := range chineseInspectLabels {
		replacements = append(replacements, english, chinese)
	}
	return strings.NewReplacer(replacements...).Replace(value)
}

// ReplaceDiagnostic localizes fixed application diagnostics while leaving
// dynamic paths, values, and external error causes intact.
func (localizer Localizer) ReplaceDiagnostic(value string) string {
	if !localizer.IsChineseSimplified() {
		return value
	}
	replacements := make([]string, 0, len(chineseDiagnostics)*2)
	for english, chinese := range chineseDiagnostics {
		replacements = append(replacements, english, chinese)
	}
	return strings.NewReplacer(replacements...).Replace(value)
}

// Text renders one message template with its values.
func (localizer Localizer) Text(key Key, values ...any) string {
	template, ok := localizer.messages[key]
	if !ok {
		template = english[key]
	}
	return fmt.Sprintf(template, values...)
}

// BatchSummary renders the common batch result line without changing JSON reports.
func (localizer Localizer) BatchSummary(scanned, processed, skipped, failed int) string {
	if localizer.IsChineseSimplified() {
		return fmt.Sprintf("已扫描=%d 已处理=%d 已跳过=%d 已失败=%d", scanned, processed, skipped, failed)
	}
	return fmt.Sprintf("scanned=%d processed=%d skipped=%d failed=%d", scanned, processed, skipped, failed)
}

func (localizer Localizer) ProgressSent(path string) string {
	if localizer.IsChineseSimplified() {
		return fmt.Sprintf("已发送 %s", path)
	}
	return fmt.Sprintf("sent %s", path)
}

func (localizer Localizer) ProgressRetrying(path string, attempt int) string {
	if localizer.IsChineseSimplified() {
		return fmt.Sprintf("正在重试 %s 次数=%d", path, attempt)
	}
	return fmt.Sprintf("retrying %s attempt=%d", path, attempt)
}

func (localizer Localizer) ProgressFailed(path string, err error) string {
	if localizer.IsChineseSimplified() {
		return fmt.Sprintf("发送失败 %s：%v", path, err)
	}
	return fmt.Sprintf("failed %s: %v", path, err)
}

var english = map[Key]string{
	ConfigValid:           "Configuration is valid",
	ValidationValid:       "valid",
	EchoSucceeded:         "C-ECHO succeeded: %s:%d",
	RootShort:             "DICOM command-line utility",
	RootLong:              "dicom-cli provides configuration, rules, file processing, and DIMSE operations. Use a subcommand's --help output for its input, output, and safety constraints.",
	InspectShort:          "Inspect a single DICOM file",
	InspectLong:           "Inspect one DICOM file and report patient, study, series, instance, and pixel metadata. Inspection never modifies the source file. Use --tag or --profile to select additional elements.",
	HelpUsage:             "Usage:",
	HelpAliases:           "Aliases:",
	HelpExamples:          "Examples:",
	HelpAvailableCommands: "Available Commands:",
	HelpFlags:             "Flags:",
	HelpGlobalFlags:       "Global Flags:",
	HelpAdditionalTopics:  "Additional help topics:",
	HelpMoreInformation:   "Use \"%s [command] --help\" for more information about a command.",
	FlagHelp:              "help for this command",
	FlagConfig:            "configuration file",
	FlagRules:             "rules file",
	FlagVerbose:           "enable debug logging",
	FlagQuiet:             "only log errors",
	FlagLogFormat:         "log format: text or json",
	FlagInspectAll:        "include every data element",
	FlagInspectTag:        "DICOM keyword or hexadecimal Tag path",
	FlagInspectProfile:    "inspect profile from rules",
	FlagJSON:              "write JSON",
	FlagReportOutput:      "report output path",
}

var chineseSimplified = map[Key]string{
	ConfigValid:           "配置有效",
	ValidationValid:       "有效",
	EchoSucceeded:         "C-ECHO 成功：%s:%d",
	RootShort:             "DICOM 命令行工具",
	RootLong:              "dicom-cli 提供配置、规则、文件处理和 DIMSE 操作。使用子命令的 --help 查看输入、输出和安全限制。",
	InspectShort:          "查看单个 DICOM 文件",
	InspectLong:           "查看一个 DICOM 文件，并报告患者、检查、序列、实例和像素元数据。查看操作不会修改源文件。使用 --tag 或 --profile 选择附加元素。",
	HelpUsage:             "用法：",
	HelpAliases:           "别名：",
	HelpExamples:          "示例：",
	HelpAvailableCommands: "可用命令：",
	HelpFlags:             "选项：",
	HelpGlobalFlags:       "全局选项：",
	HelpAdditionalTopics:  "其他帮助主题：",
	HelpMoreInformation:   "使用 \"%s [command] --help\" 查看命令详情。",
	FlagHelp:              "显示此命令的帮助",
	FlagConfig:            "运行配置文件",
	FlagRules:             "规则文件",
	FlagVerbose:           "启用 debug 日志",
	FlagQuiet:             "仅输出 error 日志",
	FlagLogFormat:         "日志格式：text 或 json",
	FlagInspectAll:        "包含所有数据元素",
	FlagInspectTag:        "DICOM 关键字或十六进制 Tag 路径",
	FlagInspectProfile:    "规则中的查看 profile",
	FlagJSON:              "输出 JSON",
	FlagReportOutput:      "报告输出路径",
}

var englishCommands = map[string]CommandText{
	"dicom-cli config":               {Short: "Manage runtime configuration", Long: "Create, validate, and maintain the runtime configuration used by DIMSE commands. Configuration discovery selects one file; it never merges multiple configuration files."},
	"dicom-cli lang":                 {Short: "Set the persistent CLI language", Long: "Set the language in the selected existing configuration file. Subsequent commands that use this configuration display human-readable output in the selected language."},
	"dicom-cli config init":          {Short: "Create a runtime configuration example", Long: "Create a YAML or JSON runtime configuration example. Existing files are never overwritten unless --force is supplied."},
	"dicom-cli config language":      {Short: "Set the persistent CLI language", Long: "Set the language in the selected existing configuration file. Subsequent commands that use this configuration display human-readable output in the selected language."},
	"dicom-cli config validate":      {Short: "Validate a runtime configuration", Long: "Validate one runtime configuration file. When no path is provided, normal configuration discovery is used and built-in defaults are validated when no file is found."},
	"dicom-cli config target":        {Short: "Manage named PACS targets", Long: "List and modify named PACS targets. Changes are made to the selected configuration file, which must already exist."},
	"dicom-cli config target list":   {Short: "List named PACS targets", Long: "List the named PACS targets in the selected configuration file. Output contains one name per line."},
	"dicom-cli config target add":    {Short: "Add a named PACS target", Long: "Add a named PACS target to the selected configuration file. This command requires all four connection fields: --host, --port, --calling-ae, and --called-ae."},
	"dicom-cli config target update": {Short: "Update a named PACS target", Long: "Update an existing named PACS target. Only explicitly supplied fields are changed; omitted fields retain their configured values."},
	"dicom-cli config target remove": {Short: "Remove a named PACS target", Long: "This command removes the target from the selected configuration file. It fails when the target does not exist."},
	"dicom-cli rules":                {Short: "Manage DICOM rules", Long: "Rule files provide named filters, inspection profiles, anonymization profiles, validation profiles, and DICOM templates."},
	"dicom-cli rules init":           {Short: "Create a rules example", Long: "Create a YAML or JSON rules example. Existing files are never overwritten unless --force is supplied."},
	"dicom-cli rules validate":       {Short: "Validate a rules file", Long: "Validate one rules file selected by its path or normal rules discovery. Unknown fields are rejected so misspelled rule names cannot be ignored."},
	"dicom-cli inspect":              {Short: "Inspect a single DICOM file", Long: "Inspect one DICOM file and report patient, study, series, instance, and pixel metadata. Inspection never modifies the source file. Use --tag or --profile to select additional elements."},
	"dicom-cli validate":             {Short: "Validate a single DICOM file", Long: "Validate one DICOM file and report all independent findings. Errors return the DICOM validation exit code; --strict also treats warnings as failures."},
	"dicom-cli edit":                 {Short: "Edit one DICOM file into a new file", Long: "Apply tag edits to one DICOM file and always write a new output file. At least one edit operation is required. Private or unknown tags require --vr TagPath=VR when their VR cannot be inferred."},
	"dicom-cli anonymize":            {Short: "Anonymize DICOM files into new files", Long: "Anonymize one DICOM file or a directory using the Basic Application Level Confidentiality Profile and optional rules. UID mappings are shared across the batch so related instances retain consistent replacement UIDs. Use --report only in a protected location because it can contain sensitive before-and-after values."},
	"dicom-cli convert":              {Short: "Export DICOM images and metadata", Long: "DICOM input is exported either as rendered image frames or as metadata JSON. Select the image or json subcommand; conversion never rewrites the source DICOM file."},
	"dicom-cli convert image":        {Short: "Export DICOM pixel data as PNG or JPEG", Long: "Export DICOM pixel data as PNG or JPEG. Frame numbers start at 1; without --frame or --all-frames, the first frame is exported. Binary stdout requires exactly one result."},
	"dicom-cli convert json":         {Short: "Export DICOM metadata as JSON", Long: "Export DICOM metadata as JSON. PixelData is summarized by default; use --include-pixel-data only when the full pixel bytes are required."},
	"dicom-cli encapsulate":          {Short: "Encapsulate external content as DICOM", Long: "External images are imported into uncompressed Secondary Capture DICOM files. Select the image subcommand to provide metadata and output handling."},
	"dicom-cli encapsulate image":    {Short: "Encapsulate PNG or JPEG images as Secondary Capture DICOM", Long: "Encapsulate supported PNG or JPEG images as uncompressed Explicit VR Little Endian Secondary Capture DICOM. PatientName must come from --patient-name, --template, or --reference. For directory input, Study and Series UIDs are shared while each image receives a distinct SOP Instance UID."},
	"dicom-cli transcode":            {Short: "Re-encode DICOM transfer syntaxes", Long: "Re-encode DICOM transfer syntaxes.\n\n--to accepts a transfer syntax alias or standard UID. Run `dicom-cli transcode formats` to list values available in this binary. Both --to and --output are required, and the source file is never overwritten."},
	"dicom-cli transcode formats":    {Short: "List transfer syntaxes available in this binary", Long: "List transfer syntax aliases, UIDs, and encode/decode capabilities registered in this binary. Use an alias or UID from this output with transcode --to."},
	"dicom-cli echo":                 {Short: "Verify a DIMSE target with C-ECHO", Long: "Open a DIMSE Association and issue one C-ECHO request. C-ECHO verifies reachability and negotiation but does not modify remote data. Select --target or provide the connection overrides directly."},
	"dicom-cli send":                 {Short: "Send DICOM instances with C-STORE", Long: "Send DICOM instances with C-STORE from one file, a directory, or newline-delimited paths on stdin. The command reuses Associations when possible and does not transcode source instances; transcode before sending when the target cannot accept the source transfer syntax."},
}

var chineseCommandShort = map[string]string{
	"dicom-cli anonymize":            "将 DICOM 文件脱敏后写入新文件",
	"dicom-cli config":               "管理运行配置",
	"dicom-cli lang":                 "设置持久 CLI 语言",
	"dicom-cli config init":          "创建运行配置示例",
	"dicom-cli config language":      "设置持久 CLI 语言",
	"dicom-cli config validate":      "校验运行配置",
	"dicom-cli config target":        "管理命名 PACS 目标",
	"dicom-cli config target list":   "列出命名 PACS 目标",
	"dicom-cli config target add":    "添加命名 PACS 目标",
	"dicom-cli config target update": "更新命名 PACS 目标",
	"dicom-cli config target remove": "删除命名 PACS 目标",
	"dicom-cli convert":              "导出 DICOM 图像和元数据",
	"dicom-cli convert image":        "将 DICOM 像素数据导出为 PNG 或 JPEG",
	"dicom-cli convert json":         "将 DICOM 元数据导出为 JSON",
	"dicom-cli echo":                 "通过 C-ECHO 验证 DIMSE 目标",
	"dicom-cli edit":                 "编辑一个 DICOM 文件并写入新文件",
	"dicom-cli encapsulate":          "将外部内容封装为 DICOM",
	"dicom-cli encapsulate image":    "将 PNG 或 JPEG 图像封装为 Secondary Capture DICOM",
	"dicom-cli inspect":              "查看单个 DICOM 文件",
	"dicom-cli rules":                "管理 DICOM 规则",
	"dicom-cli rules init":           "创建规则示例",
	"dicom-cli rules validate":       "校验规则文件",
	"dicom-cli send":                 "通过 C-STORE 发送 DICOM 实例",
	"dicom-cli transcode":            "重编码 DICOM 传输语法",
	"dicom-cli transcode formats":    "列出当前二进制可用的传输语法",
	"dicom-cli validate":             "校验单个 DICOM 文件",
}

var chineseCommandLong = map[string]string{
	"dicom-cli config":               "创建、校验和维护 DIMSE 命令使用的运行配置。配置发现只选择一个文件，不会合并多个配置文件。",
	"dicom-cli lang":                 "在所选且已存在的配置文件中设置语言。后续使用该配置的命令会以所选语言显示面向用户的输出。",
	"dicom-cli config init":          "创建 YAML 或 JSON 运行配置示例。除非提供 --force，否则绝不覆盖已有文件。",
	"dicom-cli config language":      "在所选且已存在的配置文件中设置语言。后续使用该配置的命令会以所选语言显示面向用户的输出。",
	"dicom-cli config validate":      "校验一个运行配置文件。未提供路径时使用常规配置发现；未找到文件时校验内置默认值。",
	"dicom-cli config target":        "列出和修改命名 PACS 目标。修改写入所选配置文件，该文件必须已存在。",
	"dicom-cli config target list":   "列出所选配置文件中的命名 PACS 目标，每行一个名称。",
	"dicom-cli config target add":    "向所选配置文件添加命名 PACS 目标。必须提供 --host、--port、--calling-ae 和 --called-ae。",
	"dicom-cli config target update": "更新已有命名 PACS 目标。只修改显式提供的字段，未提供字段保留原配置值。",
	"dicom-cli config target remove": "从所选配置文件删除命名 PACS 目标；目标不存在时失败。",
	"dicom-cli rules":                "规则文件提供命名筛选器、查看 profile、脱敏 profile、校验 profile 和 DICOM 模板。",
	"dicom-cli rules init":           "创建 YAML 或 JSON 规则示例。除非提供 --force，否则绝不覆盖已有文件。",
	"dicom-cli rules validate":       "按路径或常规规则发现校验规则文件。拒绝未知字段，避免忽略拼写错误的规则名。",
	"dicom-cli inspect":              "查看一个 DICOM 文件，并报告患者、检查、序列、实例和像素元数据。查看操作不会修改源文件。使用 --tag 或 --profile 选择附加元素。",
	"dicom-cli validate":             "校验一个 DICOM 文件并报告所有独立问题。错误返回 DICOM 校验退出码；--strict 也会将 warning 视为失败。",
	"dicom-cli edit":                 "对一个 DICOM 文件应用 Tag 编辑，并始终写入新输出文件。至少需要一个编辑操作。无法推断 VR 的私有或未知 Tag 需要 --vr TagPath=VR。",
	"dicom-cli anonymize":            "使用 Basic Application Level Confidentiality Profile 和可选规则对一个 DICOM 文件或目录脱敏。UID 映射在同一批处理内共享，使相关实例保持一致的替换 UID。--report 可能包含敏感前后值，只能写入受保护位置。",
	"dicom-cli convert":              "将 DICOM 输入导出为渲染图像帧或元数据 JSON。选择 image 或 json 子命令；转换不会改写源 DICOM 文件。",
	"dicom-cli convert image":        "将 DICOM 像素数据导出为 PNG 或 JPEG。帧号从 1 开始；未提供 --frame 或 --all-frames 时导出首帧。二进制标准输出要求恰好一个结果。",
	"dicom-cli convert json":         "将 DICOM 元数据导出为 JSON。默认仅汇总 PixelData；只有需要完整像素字节时才使用 --include-pixel-data。",
	"dicom-cli encapsulate":          "将外部图像导入未压缩 Secondary Capture DICOM 文件。选择 image 子命令提供元数据和输出处理。",
	"dicom-cli encapsulate image":    "将支持的 PNG 或 JPEG 图像封装为未压缩 Explicit VR Little Endian Secondary Capture DICOM。PatientName 必须来自 --patient-name、--template 或 --reference。目录输入时共享 Study 和 Series UID，每张图像获得独立 SOP Instance UID。",
	"dicom-cli transcode":            "重编码 DICOM 传输语法。--to 接受传输语法别名或标准 UID；运行 dicom-cli transcode formats 可列出当前二进制可用值。--to 和 --output 均为必填，且绝不覆盖源文件。",
	"dicom-cli transcode formats":    "列出当前二进制注册的传输语法别名、UID 及编解码能力。使用此输出中的别名或 UID 作为 transcode --to 的值。",
	"dicom-cli echo":                 "打开 DIMSE Association 并发起一次 C-ECHO 请求。C-ECHO 验证可达性和协商，但不会修改远程数据。选择 --target 或直接提供连接覆盖参数。",
	"dicom-cli send":                 "从一个文件、目录或标准输入的逐行路径通过 C-STORE 发送 DICOM 实例。命令会尽可能复用 Association，且不会转码源实例；目标无法接受源传输语法时请先转码。",
}

var chineseFlagUsage = map[string]string{
	"all":                "包含所有数据元素",
	"all-frames":         "导出全部图像帧",
	"associate-timeout":  "Association 协商超时",
	"called-ae":          "被叫 AE Title 覆盖值",
	"calling-ae":         "主叫 AE Title 覆盖值",
	"charset":            "输出字符集",
	"clear":              "TagPath",
	"concurrency":        "最大并发 Association 数",
	"connect-timeout":    "TCP 连接超时",
	"delete":             "TagPath",
	"fail-fast":          "首个文件失败后停止",
	"failed-list":        "将失败路径写为每行一个的列表",
	"filter":             "目录输入使用的命名规则筛选器",
	"flatten":            "不保留输入目录结构",
	"force":              "覆盖已有文件",
	"format":             "输出格式",
	"frame":              "从 1 开始的帧号",
	"generate-uid":       "UID TagPath",
	"host":               "PACS 主机覆盖值",
	"idle-timeout":       "DIMSE 读写空闲超时",
	"include-pixel-data": "包含 PixelData 字节",
	"input-charset":      "覆盖输入字符集",
	"json":               "输出 JSON",
	"max-instances":      "每个 Association 的最大实例数（0 为不限）",
	"option":             "标准脱敏 profile 选项",
	"output":             "输出路径",
	"patient-name":       "创建 DICOM 所需的 PatientName",
	"port":               "PACS 端口覆盖值",
	"profile":            "规则中的 profile",
	"recursive":          "扫描子目录",
	"reference":          "参考 DICOM 元数据来源",
	"remap-uids":         "重映射文件中的全部 UID 值",
	"report":             "写入详细 JSON 报告",
	"retries":            "网络和超时失败的重试次数",
	"set":                "TagPath=value",
	"strict":             "将 warning 视为失败",
	"tag":                "DICOM 关键字或十六进制 Tag 路径",
	"target":             "命名 PACS 目标",
	"template":           "规则中的命名 DICOM 模板",
	"to":                 "目标传输语法别名或 UID",
	"uid-root":           "生成 UID 使用的 UID 根",
	"vr":                 "私有或未知 Tag 使用 TagPath=VR",
}

var chineseInspectLabels = map[string]string{
	"[File]":                        "[文件]",
	"[Patient]":                     "[患者]",
	"[Study]":                       "[检查]",
	"[Series]":                      "[序列]",
	"[Instance]":                    "[实例]",
	"[Pixel]":                       "[像素]",
	"[Elements]":                    "[数据元素]",
	"[Selected Tags]":               "[选定 Tag]",
	"  Path:":                       "  路径：",
	"  Transfer Syntax:":            "  传输语法：",
	"  Name:":                       "  姓名：",
	"  ID:":                         "  ID：",
	"  Birth Date:":                 "  出生日期：",
	"  Sex:":                        "  性别：",
	"  Instance UID:":               "  实例 UID：",
	"  Modality:":                   "  模态：",
	"  Date:":                       "  日期：",
	"  Time:":                       "  时间：",
	"  Accession Number:":           "  检查编号：",
	"  Description:":                "  描述：",
	"  Referring Physician:":        "  转诊医师：",
	"  Number:":                     "  编号：",
	"  Body Part:":                  "  身体部位：",
	"  Laterality:":                 "  方位：",
	"  Protocol:":                   "  协议：",
	"  SOP Class UID:":              "  SOP Class UID：",
	"  SOP Instance UID:":           "  SOP Instance UID：",
	"  Image Position:":             "  图像位置：",
	"  Image Orientation:":          "  图像方向：",
	"  Slice Thickness:":            "  层厚：",
	"  Spacing Between Slices:":     "  层间距：",
	"  Rows:":                       "  行：",
	"  Columns:":                    "  列：",
	"  Frames:":                     "  帧数：",
	"  Bytes:":                      "  字节数：",
	"  Samples Per Pixel:":          "  每像素样本数：",
	"  Photometric Interpretation:": "  光度解释：",
	"  Bits Allocated:":             "  分配位数：",
	"  Bits Stored:":                "  存储位数：",
	"  High Bit:":                   "  高位：",
	"  Pixel Representation:":       "  像素表示：",
	"  Pixel Spacing:":              "  像素间距：",
	"  Window Center:":              "  窗位：",
	"  Window Width:":               "  窗宽：",
}

var chineseDiagnostics = map[string]string{
	"--verbose and --quiet cannot be used together": "--verbose 和 --quiet 不能同时使用",
	"invalid command arguments":                     "命令参数无效",
	"unexpected arguments:":                         "不应包含的位置参数：",
	"output path is the input path":                 "输出路径不能与输入路径相同",
	"binary stdout requires exactly one result":     "二进制标准输出要求恰好一个结果",
	"binary stdout requires exactly one input file": "二进制标准输出要求恰好一个输入文件",
	"output file":    "输出文件",
	"already exists": "已存在",
	"is a directory; this command requires one DICOM file": "是目录；此命令要求一个 DICOM 文件",
	"is required": "为必填项",
}
