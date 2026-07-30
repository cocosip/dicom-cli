# dicom-cli 使用手册

本手册描述当前 `dicom-cli` 二进制的命令契约。运行 `dicom-cli <命令> --help`
可以查看与所用版本完全一致的帮助信息。

## 全局参数与日志

所有业务命令都接受以下全局参数：

| 参数 | 说明 |
| --- | --- |
| `-c, --config <path>` | 指定运行配置文件。 |
| `-R, --rules <path>` | 指定规则文件。 |
| `-v, --verbose` | 将日志级别提高到 debug。 |
| `-q, --quiet` | 只输出 error 级别日志。 |
| `--log-format text|json` | 选择 stderr 日志格式，默认 `text`。 |

`--verbose` 与 `--quiet` 不能同时使用。业务结果写入 stdout，日志和 `send`
进度写入 stderr；调用脚本应按此分流。运行配置使用命令行覆盖、
`DICOM_CLI_*` 环境变量、配置文件和默认值的优先级。

## 配置与规则

`config init [path]` 生成 YAML 配置；加 `--format json` 生成 JSON，已有文件只有
传 `--force` 才允许覆盖。`config validate [path]` 校验配置。未传路径时，配置查找
优先当前目录，其次用户配置目录；`--config` 指定路径优先。

命名 PACS 目标通过以下命令维护：

```sh
dicom-cli config target add local-pacs \
  --host pacs.example.test --port 11112 \
  --calling-ae DICOMCLI --called-ae PACS
dicom-cli config target list
dicom-cli config target update local-pacs --host pacs.example.test
dicom-cli config target remove local-pacs
```

`config target add` 必须同时提供 `--host`、`--port`、`--calling-ae` 和
`--called-ae`。`update` 只更新实际传入的字段。可从
[`examples/dicom-cli.yaml`](../examples/dicom-cli.yaml) 复制完整示例。

`rules init [path]` 和 `rules validate [path]` 的覆盖与格式行为和 `config` 相同。
规则文件包含 `filters`、`inspect.profiles`、`anonymize.profiles`、
`validate.profiles` 与 `dicom_templates`；未知字段会被拒绝。完整示例见
[`examples/dicom-cli-rules.yaml`](../examples/dicom-cli-rules.yaml)。

## inspect、validate 与 edit

### inspect

```sh
dicom-cli inspect image.dcm
dicom-cli inspect --tag PatientName --tag 0040,A730[0].0040,A160 image.dcm
dicom-cli inspect --profile summary --rules dicom-cli-rules.yaml image.dcm
dicom-cli inspect --all --json --output report.json image.dcm
```

`inspect` 只接受一个 DICOM 文件。默认输出患者、检查、序列和像素摘要；
`--all` 输出所有数据元素，`--tag` 可重复指定 DICOM 关键字或十六进制 Tag 路径，
`--profile` 从规则文件选择标签清单。`--json` 输出 JSON，`--output` 将报告写入文件。

### validate

```sh
dicom-cli validate image.dcm
dicom-cli validate --strict --json --output validation.json image.dcm
dicom-cli validate --profile ct-required-identifiers --rules dicom-cli-rules.yaml image.dcm
```

`validate` 只接受一个 DICOM 文件。它会收集多个独立问题；默认 warning 不改变
退出码，`--strict` 会将 warning 也按校验失败处理。`--profile` 叠加规则文件的
命名校验 profile。

### edit

```sh
dicom-cli edit image.dcm --set PatientName=ANON^PATIENT --output edited.dcm
dicom-cli edit image.dcm --clear PatientID --delete AccessionNumber --output edited.dcm
dicom-cli edit image.dcm --generate-uid StudyInstanceUID --uid-root 1.2.156.112618 --output edited.dcm
```

`edit` 只接受一个文件并始终写出新文件。`--set` 使用 `TagPath=value`，`--clear`、
`--delete` 和 `--generate-uid` 接受可重复的 TagPath；标准 Tag 自动推断 VR，私有
或未知 Tag 需要 `--vr TagPath=VR`。`--remap-uids` 重映射文件中的 UID 值。
`--charset` 设置输出字符集，`--input-charset` 覆盖输入字符集解释。输入路径不能
与输出路径相同。

## anonymize

```sh
dicom-cli anonymize --profile basic --output anonymized image.dcm
dicom-cli anonymize --profile research --rules dicom-cli-rules.yaml --output anonymized study
dicom-cli anonymize --profile basic --recursive --filter ct-images --output anonymized study
```

`anonymize` 接受单文件或目录，目录默认只扫描当前层，`--recursive` 才进入子目录。
默认使用内置 `basic` profile；`--profile` 可选规则文件中的命名 profile，
`--option` 可重复指定标准 profile 选项。支持的选项为：

- `retain-safe-private`
- `retain-uids`
- `retain-device-identity`
- `retain-institution-identity`
- `retain-patient-characteristics`
- `retain-longitudinal-temporal-information-with-full-dates`
- `retain-longitudinal-temporal-information-with-modified-dates`
- `clean-descriptors`
- `clean-structured-content`
- `clean-graphics`

目录输出默认保留输入相对层级；`--flatten` 改为平铺并自动处理重名。`--filter`
仅用于目录输入。`--fail-fast` 在首个文件失败时停止，默认继续处理其余文件。
`--report` 写出可能包含敏感前后值和 UID 映射的详细报告，应妥善保护；`--json`
只输出汇总。单文件可以用 `--output -` 将二进制 DICOM 写到 stdout，多个二进制结果
不能写到 stdout。

## convert 与 transcode

### convert

```sh
dicom-cli convert image --format png --output image.png image.dcm
dicom-cli convert image --all-frames --output frames image.dcm
dicom-cli convert json --output metadata.json image.dcm
dicom-cli convert dicom --patient-name ANON^PATIENT --output image.dcm source.png
```

`convert image <input>` 导出 PNG 或 JPEG。`--frame` 是从 1 开始的帧号；默认导出
首帧，`--all-frames` 导出每一帧。`convert json <input>` 默认将 PixelData 写为摘要，
只有 `--include-pixel-data` 才包含像素字节。`convert dicom <input>` 将 8 位灰度、
8 位 RGB PNG/JPEG 或 16 位灰度 PNG 封装成 Secondary Capture DICOM；必须由
`--patient-name`、`--template` 或 `--reference` 提供 PatientName。

三个子命令都支持文件或目录输入、`--recursive`、`--flatten`、`--fail-fast` 和
`--output`。目录默认不递归。图片和 DICOM 的二进制 stdout 都要求恰好一个结果；
多帧、多文件和目录输入不得使用 `--output -`。

顶层 `convert <input> --to png|jpeg|json|dicom` 与对应子命令走同一行为；
`--to dicom` 使用 `--patient-name`、`--template` 和 `--reference`。

### transcode

```sh
dicom-cli transcode formats
dicom-cli transcode --to rle --output compressed.dcm image.dcm
dicom-cli transcode --to 1.2.840.10008.1.2.1 --output output study
```

`transcode formats` 显示当前二进制实际注册的传输语法、别名和编码/解码能力，
可加 `--json`。`transcode <file> --to <alias-or-uid>` 接受单文件或目录；目录模式
支持 `--recursive`、`--flatten`、`--filter`、`--fail-fast`。`--to` 可使用别名或
标准传输语法 UID。转码只改变与编解码和传输语法有关的数据；输出路径不得是输入路径。

HTJ2K 显示为 experimental。能列出或完成合成样本转码不等于已验证真实样本互操作。

## echo 与 send

目标可由 `--target <name>` 从配置中选择，或由以下参数覆盖：`--host`、`--port`、
`--calling-ae`、`--called-ae`、`--connect-timeout`、`--associate-timeout` 和
`--idle-timeout`。首版只使用明文 TCP。默认连接超时为 10 秒、Association 协商超时
为 30 秒、读写空闲超时为 5 分钟。

```sh
dicom-cli echo --target local-pacs --config dicom-cli.yaml
dicom-cli echo --host pacs.example.test --port 11112 --calling-ae DICOMCLI --called-ae PACS
dicom-cli send --target local-pacs --config dicom-cli.yaml --recursive study
```

`echo` 执行一次 C-ECHO，`--json` 输出 JSON 结果。`send` 接受单文件、目录或 `-`；
输入为 `-` 时从 stdin 读取每行一个文件路径。`send` 的进度写 stderr，stdout 输出
汇总或 `--json` 汇总。`--report` 写详细 JSON，`--failed-list` 写失败路径清单，该清单
可作为下一次 `send -` 的 stdin。

`send` 默认顺序复用一个 Association。`--max-instances` 限制单个 Association 中的
实例数，`--concurrency` 控制并行 Association 数，`--retries` 只重试网络、超时或
Association 中断。PACS 返回的 C-STORE 状态失败不会重试。目标不接受源传输语法时，
`send` 不会隐式转码；请先执行 `transcode`。

## 批处理、输出与退出码

`anonymize`、`convert`、`transcode` 和目录模式 `send` 中，不符合规则筛选的文件为
正常跳过；损坏或非 DICOM 文件会计为失败。默认批处理继续处理剩余文件，
`--fail-fast` 会在首个失败后停止。

| 退出码 | 含义 |
| --- | --- |
| `0` | 成功。 |
| `1` | 操作失败，包括批处理中的失败文件、网络和 DIMSE 失败。 |
| `2` | 参数、配置或规则输入不合法。 |
| `3` | DICOM 校验失败。 |

真实样本/PACS 验收不在默认命令或 CI 中执行。运行受控外部验收前，应确认样本可合法
使用、PACS 目标已获授权，并将地址、AE Title 和凭据保留在部署环境而非仓库中。
