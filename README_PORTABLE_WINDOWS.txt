FGO Calc Local Box - Windows Portable
=====================================

普通用户使用方法
----------------
1. 完整解压本 ZIP 到一个普通文件夹。
2. 双击 “Start FGO Calc.bat”。
3. 等待浏览器自动打开 http://127.0.0.1:30006 。
4. 使用期间请保持启动窗口开启；关闭窗口即可停止本地服务。

不需要安装 Go
-------------
Windows Portable 已经包含预编译的 Go 后端，因此正常使用不需要 Go、编译器或其他开发环境。

数据更新是可选功能
------------------
Release 自带一份可直接使用的游戏数据。
只有在需要使用“检查更新 / 更新游戏数据”时，才需要额外安装：
- Python 3.10+
- Git for Windows

如果没有安装 Python / Git，计算器本体仍可继续使用 Release 自带的数据。

隐私与本地边界
--------------
- 后端只监听 127.0.0.1:30006，不向局域网开放。
- 个人 Box / Planner 状态主要保存在浏览器 localStorage。
- FGO / Stream toplogin Response 在浏览器本地解析；请勿把原始抓包文件公开上传。
- Data Sync 需要联网获取公开游戏数据，但不会把个人 Box 上传到项目方服务器。

Windows 安全提示
----------------
本项目没有购买商业代码签名证书，因此 Windows 可能对首次下载的可执行文件显示未知发布者 / SmartScreen 提示。
请只从 AkatuRegumi/fgo-calc 的 GitHub Releases 下载，并可使用同 Release 提供的 SHA256SUMS.txt 校验 ZIP。

源码与上游
----------
本项目是 pikaball/fgo-calc 的本地增强 fork，并继续按仓库 LICENSE 的 GPL-3.0 条款发布。
详细功能、来源与许可证信息请查看 README.md、README_LOCAL_BOX.md 与 LICENSE。
