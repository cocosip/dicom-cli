# dicom-cli 开发进度

本文件是唯一的开发执行台账。每个开发项均使用标准 Markdown 任务列表，完成后将 `- [ ]` 改为 `- [x]`；只有完成条件有真实证据时才可勾选。需求范围见 [需求规格](requirements.md)，决策背景见 [讨论稿](dicom-cli-discussion.md)。

## 文档基线

- [x] **D0.1 产品范围讨论**：已完成；见 [讨论稿](dicom-cli-discussion.md)。
- [x] **D0.2 需求规格**：已完成；见 [需求规格](requirements.md)。
- [x] **D0.3 开发执行台账**：已完成；本文件为后续唯一进度入口。
- [x] **D0.4 Go 工程初始化**：已创建 `go.mod`、Go 源码和测试样本。

## P0 工程骨架与公共契约

- [x] **P0.1 初始化 Go 模块与直接依赖**：依赖 D0.3；完成条件：`go.mod` 声明 Go 1.26、Cobra、Viper、go-dicom、go-dicom-codecs，`go mod tidy` 和 `go list -m all` 成功。
- [x] **P0.2 建立可执行入口**：依赖 P0.1；完成条件：存在 `cmd/dicom-cli/main.go`，`go build ./cmd/dicom-cli` 成功。
- [x] **P0.3 定义应用错误与退出码**：依赖 P0.1；完成条件：处理失败映射 1、参数/配置/规则错误映射 2、校验失败映射 3，且单元测试覆盖全部映射。
- [x] **P0.4 建立根 Cobra 命令和全局参数**：依赖 P0.2、P0.3；完成条件：`-c/-R/-v/-q/--log-format` 可用，`go run ./cmd/dicom-cli --help` 正确展示。
- [x] **P0.5 接入 slog 日志**：依赖 P0.4；完成条件：支持 info/debug/error 和 JSON 格式，流测试证明日志固定写入 stderr。
- [x] **P0.6 建立命令测试设施**：依赖 P0.4；完成条件：测试可注入 stdin/stdout/stderr、工作目录和环境变量，不依赖真实控制台或用户目录。
- [x] **P0.7 验收 P0**：依赖 P0.1-P0.6；完成条件：`go test ./cmd/dicom-cli ./internal/apperr` 与 `go vet ./cmd/dicom-cli ./internal/apperr` 成功。

## P1 运行配置与规则文件

- [x] **P1.1 定义运行配置模型**：依赖 P0.1；完成条件：支持版本、UID 根、命名 PACS、超时及预留 TLS/代理/认证字段，非法端口和 AE Title 有单元测试。
- [x] **P1.2 实现运行配置发现**：依赖 P1.1、P0.6；完成条件：覆盖 `--config`、当前目录优先、用户目录回退且不合并的表驱动测试通过。
- [x] **P1.3 实现配置优先级**：依赖 P1.2；完成条件：命令行、`DICOM_CLI_*`、配置和默认值的冲突优先级均有测试。
- [x] **P1.4 实现 config init/validate**：依赖 P1.1-P1.3；完成条件：YAML/JSON 示例均可生成和校验，既有文件无 `--force` 时被拒绝覆盖。
- [x] **P1.5 实现命名 PACS 目标 CRUD**：依赖 P1.4；完成条件：`config target list|add|update|remove` 的临时配置集成测试通过。
- [x] **P1.6 定义规则文件 v1 模型**：依赖 P0.1；完成条件：统一保存筛选、inspect、anonymize、validate 与 DICOM 模板，未知字段被严格拒绝。
- [x] **P1.7 实现条件 DSL**：依赖 P1.6；完成条件：Tag 路径、存在性、相等、正则、数值范围、`all`、`any` 分别有单元测试。
- [x] **P1.8 实现规则静态校验**：依赖 P1.6、P1.7；完成条件：命名引用、Tag 路径、动作参数、重复脱敏动作都校验，坏文件可一次列出全部问题。
- [x] **P1.9 实现 rules init/validate**：依赖 P1.8；完成条件：YAML/JSON 示例生成后均通过 `rules validate`。
- [x] **P1.10 验收 P1**：依赖 P1.1-P1.9；完成条件：`go test ./internal/config ./internal/rules ./cmd/dicom-cli` 成功。

## P2 DICOM 基础能力与批处理基础设施

- [x] **P2.1 建立合成 DICOM fixture 工厂**：依赖 P0.1；完成条件：提供无真实患者信息的单帧、多帧、序列、UID 引用和损坏文件 fixture，均可被内部包复用。
- [x] **P2.2 封装 DICOM 文件读写**：依赖 P2.1；完成条件：基于 go-dicom 读写文件元信息和数据集，读写测试证明源文件未被改写。
- [x] **P2.3 实现 Tag 路径读取**：依赖 P2.1、P2.2；完成条件：支持关键字、十六进制和嵌套序列索引，正反例测试通过。
- [x] **P2.4 实现 Tag VR 判定**：依赖 P2.3；完成条件：标准 Tag 可由字典取得 VR，私有/未知 Tag 缺显式 VR 时失败。
- [x] **P2.5 实现 UID 生成**：依赖 P2.1；完成条件：支持 `2.25.<UUID 十进制>` 和机构 UID 根，格式与唯一性测试通过。
- [x] **P2.6 实现单次调用 UID 映射**：依赖 P2.5；完成条件：同一原 UID 一致、不同原 UID 不合并，Study/Series/SOP 与引用 UID 测试通过。
- [x] **P2.7 实现目录扫描**：依赖 P0.6、P1.7；完成条件：支持当前层、递归、目录链接排除、文件链接处理与筛选跳过。
- [x] **P2.8 实现安全输出路径**：依赖 P2.7；完成条件：支持默认目录、保留层级、平铺、重名序号，并拒绝输入同路径输出。
- [x] **P2.9 实现输出适配**：依赖 P0.5、P2.8；完成条件：文本、JSON、单二进制 stdout 与目录输出均可用，多结果二进制 stdout 被拒绝。
- [x] **P2.10 实现批处理执行器**：依赖 P2.7、P2.9；完成条件：统计 scanned/processed/skipped/failed，保留筛选跳过原因，支持继续处理与 fail-fast。
- [x] **P2.11 验收 P2**：依赖 P2.1-P2.10；完成条件：`go test ./internal/dicom ./internal/files ./internal/output ./internal/app ./internal/testutil` 成功。

## P3 单文件命令：inspect、validate、edit

- [x] **P3.1 实现 inspect 默认摘要**：依赖 P2.2、P2.9；完成条件：输出文件、患者、检查、序列和像素摘要，合成 DICOM 快照测试通过。
- [x] **P3.2 实现 inspect 筛选能力**：依赖 P3.1、P1.7；完成条件：`--all`、关键字/十六进制 `--tag` 与规则标签 profile 均有测试。
- [x] **P3.3 完成 inspect 命令输出**：依赖 P3.1、P2.9；完成条件：文本、JSON、`--output` 可用，目录输入返回退出码 2。
- [x] **P3.4 实现 validate 结果收集器**：依赖 P2.2；完成条件：单文件多个独立问题能一次全部返回。
- [x] **P3.5 实现 validate 基础检查**：依赖 P3.4；完成条件：解析、FMI、传输语法、VR/VM 和必填值检查都包含来源、路径、级别和消息。
- [x] **P3.6 接入标准与自定义校验规则**：依赖 P3.5、P1.8；完成条件：内置 SOP Class/IOD 规则无外部文件仍执行，`validate.profiles` 正确叠加。
- [x] **P3.7 完成 validate 命令输出与退出码**：依赖 P3.4-P3.6、P2.9；完成条件：文本/JSON/文件报告可用，严格 warning 与普通模式的退出码测试通过。
- [x] **P3.8 实现 edit 基础操作**：依赖 P2.2-P2.4；完成条件：set、clear、delete、嵌套序列路径写出后重新读取断言通过。
- [x] **P3.9 实现 edit 的 VR 与 UID 操作**：依赖 P2.4-P2.6、P3.8；完成条件：标准 Tag 自动 VR、私有显式 VR、UID 生成和文件内重映射均有测试。
- [x] **P3.10 实现 edit 字符集处理**：依赖 P3.8；完成条件：`--charset` 更新 SpecificCharacterSet 并重编码文本 VR，`--input-charset` 覆盖源解释。
- [x] **P3.11 完成 edit 命令安全边界**：依赖 P3.8-P3.10、P2.8；完成条件：只接受单文件，不读取规则 profile，不允许输入同路径输出，源文件保持不变。
- [x] **P3.12 验收 P3**：依赖 P3.1-P3.11；完成条件：`go test ./internal/app ./internal/validate ./cmd/dicom-cli ./tests/integration -run 'Inspect|Validate|Edit'` 成功。

## P4 脱敏

- [x] **P4.1 定义内置脱敏 profile**：依赖 P1.6、P2.3；完成条件：实现 DICOM Basic Application Level Confidentiality Profile 及标准可选项，并有规则单元测试。
- [x] **P4.2 实现 profile 叠加与四类动作**：依赖 P4.1、P1.8；完成条件：外部规则叠加、delete、clear、replace、remap_uid 均有覆盖顺序测试。
- [x] **P4.3 接入批次 UID 映射**：依赖 P2.6、P4.2；完成条件：一次 anonymize 调用内多文件 Study/Series/SOP 与引用 UID 映射一致。
- [x] **P4.4 实现脱敏报告边界**：依赖 P4.2、P2.9；完成条件：默认摘要不含敏感原值，`--report` 文件才包含前后值和 UID 映射。
- [x] **P4.5 完成 anonymize 批处理**：依赖 P2.7-P2.10、P4.2；完成条件：文件/目录、筛选、递归、fail-fast、私有 Tag 默认保留、输出树与源文件不变均通过集成测试。
- [x] **P4.6 验收 P4 合成样本**：依赖 P4.1-P4.5；完成条件：`go test ./internal/anonymize ./internal/app ./cmd/dicom-cli -run Anonymize` 成功。
- [ ] **P4.7 脱敏真实样本回归**：依赖 P4.6；阻塞原因：等待提供可合法用于测试的脱敏真实 DICOM 样本，路径不得写死。

## P5 转换与传输语法转码

- [x] **P5.1 建立 codec 注册表**：依赖 P0.1、P2.2；完成条件：实际编译进二进制的 codec 包可枚举，传输语法别名/UID 查询不使用静态伪清单。
- [x] **P5.2 实现 transcode formats**：依赖 P5.1；完成条件：列出编码/解码能力，HTJ2K 显示 experimental 标记。
- [x] **P5.3 实现 DICOM 到 PNG/JPEG 帧导出**：依赖 P2.2、P2.9、P5.1；完成条件：默认首帧、`--frame`、`--all-frames` 与文件命名测试通过。
- [x] **P5.4 实现图片导出像素策略**：依赖 P5.3；完成条件：不应用显示 LUT/窗宽窗位，高位灰度 JPEG 缩放与 PNG 位深保留测试通过。
- [x] **P5.5 实现 DICOM 元数据 JSON/XML 与 PixelData 导出**：依赖 P2.2、P2.9；完成条件：JSON/XML 始终省略 PixelData，`convert pixeldata` 按帧顺序导出原始存储载荷。
- [x] **P5.6 统一图片/元数据/PixelData 转换入口**：依赖 P5.3-P5.5；完成条件：`convert image`、`convert json`、`convert xml`、`convert pixeldata` 仅接收 DICOM 输入并通过集成测试。
- [x] **P5.7 实现图片输入限制**：依赖 P2.2；完成条件：支持 8 位灰度、8 位 RGB PNG/JPEG 和 16 位灰度 PNG，TIFF/BMP/不支持位深失败。
- [x] **P5.8 实现图片到 DICOM 模板合并**：依赖 P1.6、P5.7；完成条件：规则模板、参考 DICOM、命令行覆盖优先级正确，缺少不可推断必填字段失败。
- [x] **P5.9 实现 Secondary Capture UID 分组**：依赖 P2.5、P5.8；完成条件：图片目录在一次调用内共享新 Study/Series UID，每图有独立 SOP UID。
- [x] **P5.10 完成图片到 DICOM 命令**：依赖 P2.8-P2.10、P5.7-P5.9；完成条件：`encapsulate image` 支持文件/目录且源图片不变，固定写出未压缩 Explicit VR Little Endian。
- [x] **P5.11 实现 transcode 文件重编码**：依赖 P5.1、P2.2；完成条件：更新 FMI 传输语法，除编解码相关数据外保留数据集内容，原始/压缩语法往返测试通过。
- [x] **P5.12 完成 transcode 批处理命令**：依赖 P2.7-P2.10、P5.11；完成条件：别名/UID、筛选、递归、fail-fast 和安全输出均通过集成测试。
- [x] **P5.13 验收 P5 合成样本**：依赖 P5.1-P5.12；完成条件：`go test ./internal/dicom ./internal/convert ./internal/app ./cmd/dicom-cli -run 'Convert|Encapsulate|Transcode'` 成功。
- [ ] **P5.14 codec 与真实 DICOM 兼容性回归**：依赖 P5.13；阻塞原因：等待外部样本，HTJ2K 需要单列实验性结果。

## P6 DIMSE SCU：echo 与 send

- [x] **P6.1 实现 DIMSE 目标解析**：依赖 P1.1-P1.3；完成条件：命名目标、命令行覆盖、环境优先级、AE/端口校验均有表驱动测试。
- [x] **P6.2 实现连接与 Association 超时**：依赖 P6.1；完成条件：明文 TCP 下连接 10 秒、协商 30 秒、读写空闲 5 分钟及覆盖值均由本地测试端验证。
- [x] **P6.3 实现 C-ECHO**：依赖 P6.2、P2.9；完成条件：本地 DIMSE peer 集成测试验证文本/JSON 结果和错误映射。
- [x] **P6.4 实现 send 输入收集**：依赖 P2.7、P6.1；完成条件：单文件、目录、stdin 路径列表及筛选跳过原因均有测试。
- [x] **P6.5 实现顺序 C-STORE 与 Association 复用**：依赖 P6.2、P6.4；完成条件：本地 peer 断言默认复用 Association，max-instances 正确滚动连接。
- [x] **P6.6 实现并行 Association 调度**：依赖 P6.5；完成条件：`--concurrency > 1` 的并行数和 Association 数量受控。
- [x] **P6.7 实现发送失败分类与重试**：依赖 P6.5；完成条件：只重试网络/超时/Association 中断，PACS C-STORE 状态失败绝不重试。
- [x] **P6.8 实现 send 进度与报告**：依赖 P2.9、P6.4；完成条件：进度写 stderr，汇总/详细报告可用，失败路径清单可被 stdin 再次消费。
- [x] **P6.9 禁止隐式转码**：依赖 P5.1、P6.5；完成条件：目标不接受源传输语法时仅报告发送失败，不调用 transcode。
- [x] **P6.10 验收 P6 本地 DIMSE 测试端**：依赖 P6.1-P6.9；完成条件：`go test ./internal/app ./cmd/dicom-cli ./tests/integration -run 'Echo|Send'` 成功。
- [ ] **P6.11 真实 PACS 互操作验收**：依赖 P6.10；阻塞原因：等待地址、端口、Calling/Called AE 和测试数据，全部通过环境变量传入。

## P7 发布与最终验收

- [x] **P7.1 编写 README**：依赖 P0-P6；已满足安装、命令、配置/规则示例、二进制输出和退出码与实际 `--help` 一致的完成条件。
  - [x] **P7.1.1 定义 README 首次运行路径**：已创建仓库根目录 `README.md`，说明 GitHub Release 下载、Windows/Linux/macOS 的五个目标包、解压后的目录结构和最小 `inspect`、`config init`、`rules init` 示例。
  - [x] **P7.1.2 编写完整命令使用手册**：已创建 `docs/usage.md`，按全局参数、配置、规则、单文件命令、批处理命令、转换/转码、DIMSE、输入输出与退出码分节；命令参数、输出和限制均以源码与 `--help` 为准。
  - [x] **P7.1.3 校核文档命令契约**：已在当前编译出的 `dicom-cli` 上运行根命令和 21 个业务子命令的 `--help`，并以全仓测试中的退出码覆盖核对 README/手册；不将 Cobra 的 `completion` 作为业务命令文档承诺。
  - [x] **P7.1.4 配置驱动中英文提示**：运行配置新增 `language: en|zh-CN`，帮助、文本报告、批处理汇总和本工具生成的诊断按所选配置本地化；`lang <en|zh-CN>` 可持久修改所选配置，使后续命令切换语言。命令/flag、JSON、退出码和 DICOM 标识保持稳定。已通过 `go test ./... -count=1`。
- [x] **P7.2 实现五平台打包**：依赖 P0.1、P7.1；已生成并校验 Windows ZIP 与 Linux/macOS tar.gz，均含可执行文件、README、完整手册、配置和规则示例。
  - [x] **P7.2.1 在发布工作流内构建归档**：`.github/workflows/release.yml` 直接使用 `go build -trimpath -buildvcs=false` 为 `windows/amd64`、`linux/amd64`、`linux/arm64`、`darwin/amd64`、`darwin/arm64` 生成二进制，并按 `dicom-cli_<version>_<goos>_<goarch>` 组装归档。
  - [x] **P7.2.2 将运行资料纳入每个归档**：发布工作流复制根目录 `README.md`、`docs/usage.md`、`examples/dicom-cli.yaml`、`examples/dicom-cli-rules.yaml`；仅 Windows 归档中的二进制使用 `dicom-cli.exe`，其余为 `dicom-cli`。
  - [x] **P7.2.3 实现归档 smoke test**：发布工作流会解压 Linux AMD64 包并运行 `dicom-cli --help`。
  - [x] **P7.2.4 固化发布归档验收**：发布工作流在 GitHub Actions 的 Ubuntu runner 中生成五个归档，避免引入独立的本地打包程序。
- [ ] **P7.3 建立 GitHub Actions CI/CD**：依赖 P7.2；完成条件：`master` 上的 Windows/Linux/macOS 执行格式检查、单元测试、合成集成测试和本机构建；`v*` Tag 创建 GitHub Release 并上传五个归档。
  - [x] **P7.3.1 建立 master CI 工作流**：已创建 `.github/workflows/ci.yml`，仅由 `push.branches: [master]` 触发，使用 `windows-latest`、`ubuntu-latest`、`macos-latest` 矩阵和 `actions/setup-go` 的 `go-version-file: go.mod`。
  - [x] **P7.3.2 固化 CI 验证命令**：已在每个 CI 矩阵目标定义 `gofmt -l` 的空输出检查、`go vet ./...`、`go test ./...` 和 `go build ./cmd/dicom-cli`。
  - [x] **P7.3.3 建立 Tag 发布工作流**：已创建 `.github/workflows/release.yml`，仅由 `push.tags: ['v*']` 触发，授予 `contents: write`，在 Ubuntu runner 上直接交叉编译、归档，并以 GitHub CLI 创建同名非草稿 Release、上传五个归档和 GitHub 自动生成的 Release Notes。
  - [x] **P7.3.4 校验工作流定义**：发布工作流只在 `v*` Tag 触发，使用 GitHub 提供的 Token，并且不包含样本路径、PACS 地址或凭据。
- [ ] **P7.4 建立可选外部测试入口**：依赖 P4.7、P5.14、P6.11；完成条件：真实样本/PACS 参数缺失时跳过，提供环境变量时执行，均不硬编码路径或地址。
  - [ ] **P7.4.1 定义外部验收环境变量与运行说明**：创建 `docs/external-validation.md`，定义 `DICOM_CLI_EXTERNAL_DICOM_DIR`、`DICOM_CLI_EXTERNAL_PACS_HOST`、`DICOM_CLI_EXTERNAL_PACS_PORT`、`DICOM_CLI_EXTERNAL_CALLING_AE` 和 `DICOM_CLI_EXTERNAL_CALLED_AE` 的用途、必填组合和运行命令。
  - [ ] **P7.4.2 实现受 build tag 保护的外部测试**：创建 `tests/external` 下的 `external_test.go`，要求 `external` build tag；缺少真实样本目录时跳过样本回归，缺少任一 PACS 参数时跳过 PACS 验收，输出文件始终写入 `t.TempDir()`。
  - [ ] **P7.4.3 验收无外部条件的跳过行为**：执行 `go test -tags=external ./tests/external` 且不设置上述变量，断言命令成功并显示跳过原因；该命令不加入常规 CI。
- [ ] **P7.5 Windows 全量验收**：依赖 P3.12、P4.6、P5.13、P6.10、P7.1-P7.3；完成条件：Windows 上 `go test ./...`、`go vet ./...`、`go build ./cmd/dicom-cli` 和 Windows ZIP 清单 smoke test 证据完整。
- [ ] **P7.6 Linux 全量验收**：依赖 P7.5；完成条件：Linux CI 上 `go test ./...`、`go vet ./...`、`go build ./cmd/dicom-cli`、五平台归档和 Linux AMD64 解压执行 smoke test 证据完整。
- [ ] **P7.7 脱敏真实样本验收**：依赖 P4.7、P7.5；阻塞原因：等待样本到位后记录执行命令、版本和结果。
- [ ] **P7.8 真实 PACS 验收**：依赖 P6.11、P7.5；阻塞原因：等待 PACS 参数到位后记录目标、测试范围和结果。
- [ ] **P7.9 第一版发布判定**：依赖 P7.5-P7.8；完成条件：全部非阻塞项完成，HTJ2K experimental 和未覆盖项写入发布说明。

## 外部依赖

- [ ] **R1 脱敏真实 DICOM 样本**：影响 P4.7、P7.7；解除条件：提供可合法测试的脱敏样本及预期覆盖范围。
- [ ] **R2 真实 PACS 集成参数**：影响 P6.11、P7.8；解除条件：提供连接参数、AE Title、认证要求和测试数据。
- [ ] **R3 IOD 标准规则覆盖边界**：影响 P3.6；解除条件：记录首版 SOP Class/IOD 覆盖范围、规则数据版本和已知缺口。
- [ ] **R4 codec 实际能力清单**：影响 P5.1-P5.14；解除条件：由 `transcode formats` 和 fixture 回归结果生成。
- [ ] **R5 HTJ2K 互操作性**：影响 P5.14、P7.9；解除条件：完成合成与真实样本验证，发布说明保留 experimental 标记。

## 更新规则

1. 开始工作时不勾选；仅在进度备注或提交说明中记录“进行中”。
2. 只有满足对应“完成条件”且有测试或验收证据时，才勾选该项。
3. 被外部条件阻塞时保持未勾选，并在任务文本中写明阻塞原因和解除条件。
4. 需求变更先更新讨论稿和需求规格，再调整受影响任务。
