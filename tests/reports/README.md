# 测试报告(本地查看)

本目录用于长期保留 Allure 报告,**全部在本地生成、离线查看**,不联网。

## 文件说明

| 路径 | 说明 | 查看是否需要工具 |
|---|---|---|
| `allure-report.html` | **单文件自包含报告** —— 双击用浏览器打开即可,离线、无依赖 | ❌ 不需要 |
| `allure-results/` | 原始测试结果(JSON + 附件),用于重新生成或累积历史趋势 | ✅ 需要 Allure CLI |
| `regen.cmd` | **一键重新生成**报告的脚本(双击运行) | 内置调用本地 CLI |

> 放进作品集 / 发给面试官时,直接用 `allure-report.html` 这一个文件即可。
> 最近一次结果:**46 用例 → 45 通过 · 1 跳过(xfail 风险标记)· 0 失败**。

## 重新生成报告(完全离线)

项目已自带 Allure CLI(`tests/tools/allure-2.29.0/`,约 40MB),配合本机 **Java 21**,
**无需联网**即可重新生成。两种方式:

### 方式一:双击 `regen.cmd`
最省事。它会用现有的 `allure-results/` 重新生成 `allure-report.html`。

### 方式二:命令行(先跑用例再生成)

```bash
cd tests
# 1) 跑用例产出结果(网关需先起来；见上级 README 的启动方式)
pytest -m "not breaker" -n auto --alluredir=reports/allure-results
pytest -m breaker -p no:xdist --alluredir=reports/allure-results

# 2) 生成单文件报告（注意 -o 指向临时目录，避免 --clean 误删 allure-results）
tools/allure-2.29.0/bin/allure generate reports/allure-results --clean --single-file -o %TEMP%/allure-out
cp %TEMP%/allure-out/index.html reports/allure-report.html

# 交互式查看（本地起服务）
tools/allure-2.29.0/bin/allure serve reports/allure-results
```

> ⚠️ 坑:`allure generate` 的 `--clean` 会清空 `-o` 指定的输出目录。
> **千万别把 `-o` 指向 `reports/`**(否则会连 `allure-results/` 一起删掉)。
> 用临时目录输出、再把单文件拷回来最安全 —— `regen.cmd` 已经这么处理。

## 附:重新下载 Allure CLI(仅当 tools/ 丢失时)

从国内镜像下载免安装包,解压到 `tests/tools/`:

```
https://maven.aliyun.com/repository/public/io/qameta/allure/allure-commandline/2.29.0/allure-commandline-2.29.0.zip
```
