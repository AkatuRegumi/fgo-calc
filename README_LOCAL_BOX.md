# FGO 最优羁绊组队求解器 — 本地 Box v9 Upstream Rebase

基于 [`pikaball/fgo-calc`](https://github.com/pikaball/fgo-calc) 的 GPL-3.0 本地增强 fork。v9 已重新对齐上游 2026-09-06 最新代码语义，同时保留 v1-v8 的个人 Box、冠位 8 加成位、FGO/Stream 导入、长期培养 Planner 与 Local-only 边界。

## v9：Upstream Rebase / GitHub Candidate

本版吸收当前上游值得保留的改动，而不重新引入其在线账号系统：

- 跟随上游正式 **GPL-3.0** License，并保留原项目来源；
- 支持 **绊15→16**：仍可获得自身羁绊，同时继续作为「夢火の導き」+25% provider；已达当前上限的绊15则自身收益为0；
- 支持 `event_party_bonuses`：部分活动从者只有实际入队时才向全队提供羁绊加成；
- 必选从者 / 自备礼装 / 助战礼装对排除集合具有显式优先级，避免 include/exclude 冲突；
- 移除旧的 Cost 推测型最小礼装数剪枝，避免合法最优组合被提前剪掉；
- 合并上游最新静态数据基线与新羁绊礼装规则；
- 保留本 fork 的 **同一百分比加算桶 + 最终取整** 语义，将礼装、活动全队 buff 与夢火一起参与真实收益计算；
- 保留 **Grand 8 加成位、司机/老板 Planner、逐从者真实收益、Box/礼装库存、Stream/toplogin 导入、Chaldea CSV、Data Sync**；
- 继续 **Local-only**：无登录/注册/JWT/SQLite 用户库，后端只监听 `127.0.0.1`，原始 FGO Response 不上传。

> v9 的覆盖包不会强行覆盖你本机已经 Data Sync 得到的 `data/*.json`。升级代码后可在页面再次点“更新游戏数据”，让当前上游数据按 v9 parser 重新生成。


## v8：Local-only cleanup

- 删除上游账号登录 / 注册 UI、JWT、Cookie 登录态和账户 API；
- 删除服务器端用户状态 / 历史记录 SQLite 存储及相关依赖（JWT、bcrypt/x/crypto、modernc SQLite）；
- Box、Planner、筛选条件继续只保存在浏览器 `localStorage`，可用 Box JSON 显式备份 / 恢复；
- 后端默认只监听 `127.0.0.1`，不再暴露到局域网其它设备；
- FGO / Stream Response 仍只在浏览器本地解析，后端不会接收原始抓包；
- `.gitignore` 继续屏蔽抓包、Box 导出、Chaldea CSV、运行时数据缓存与同步日志。
- v7→v8 覆盖升级时，新的 `build_local.bat` 会在编译前自动删除旧账号系统残留源码与 `users.db`，避免旧文件继续参与 Go 构建。

> v8 不提供云账号、云同步或服务器端历史记录。这样公开 GitHub 时，产品边界与隐私说明更加直接：数据源可联网更新，但个人 Box / 抓包解析保持本机。


## v7：FGO / Stream Account Import

“我的迦勒底”新增 **导入 FGO / Stream Response**：

- 支持完整 `toplogin` Response 的原始 JSON、URL 编码文本、Base64 文本；
- 从 `cache.replaced/updated.userSvtCollection` 读取当前持有从者、`friendshipRank`、累计 `friendship` 与 `friendshipExceedCount`；
- 自动推导当前羁绊上限（绊10→15/更高），并继续复用 v4/v5 的绊10、绊15梦火和 Planner 逻辑；
- 若之前用 Chaldea CSV 导入过 `Total(LvX)` 阈值，会保留这些静态阈值并用新的累计羁绊重新计算 `Next`；
- 从 `userSvt + userSvtStorage` 匹配当前实际持有的羁绊礼装，仅把 `limitCount >= 4` 的满破礼装写入“我的礼装”库存，避免把 4% 未满破礼装误当成 20%；
- 司机/老板、手动优先级、固定出场、培养目标全部保留，不会被抓包快照覆盖；
- 导入前显示抓包 `serverTime`。快照超过 7 天会额外提示，防止误把旧抓包覆盖到当前 Box；
- 原始响应只在浏览器 FileReader 内临时解析。Box 只持久化白名单字段；不会保存/上传 `userId`、`usk`、`sign` 等账号字段，也不会把原始响应发送给 Go 后端。

> `stream-response.txt` 一类原始抓包属于账号敏感文件，不要提交到 Git/GitHub。公开仓库建议在 `.gitignore` 继续忽略本地抓包、Box 导出和运行时数据。

## v6：游戏数据同步

页面新增 **游戏数据同步** 面板：

- 启动后自动检查 **Chaldea Data 当前 Git 提交**，并读取 Atlas Academy `/info` 作为 JP 上游版本信息；
- **检查更新**：只做轻量检查，不重建数据；有 Git 时以实际生成源 Chaldea Data 的提交号为更新门，Git 检查不可用时才退回 Atlas 版本信号；
- **更新游戏数据**：一键同步并重建：
  1. 更新 `chaldea-center/chaldea-data`；
  2. 重新生成完整从者/再临/灵衣/活动羁绊数据；
  3. 重新生成羁绊礼装；
  4. 写入 `data/source_meta.json`；
  5. 后端热重载新数据与 CE 预计算缓存；
  6. 前端重新读取数据，无需重新 build。

Data Sync 的写操作只允许来自 **localhost**，避免同局域网其他设备触发本机 Python/Git 更新流程。

### 羁绊礼装自动发现

原版 `ce.py` 需要开发者手工添加每一张新羁绊礼装。v6 改成：

- 已经人工审核过的历史礼装继续使用固定 legacy filters，避免上游 schema 变化导致旧结果静默漂移；
- 同时扫描 Chaldea/Atlas 机器数据中的 `servantFriendshipUp` function；
- 从 `svals.RateCount` 读取满破倍率；
- 从 `functvals` / target trait 读取生效特性；
- 同一礼装的普通/满破效果自动取更高值；
- 新识别的礼装自动加入 **JP** 数据集；
- 特殊旧式固定 `+50` 等效果继续由已审核 legacy contract 保底；
- 每次同步写出 `data/ce_sync_report.json`，页面会显示自动发现情况。

因此以后例如新增某职阶 `+20%` 羁绊礼装，正常情况下无需再改源码或重新发版本。

## Data Sync 环境

应用本体仍使用 Go 1.25.1+。数据生成沿用上游成熟的数据管线，因此一键更新额外需要：

- Python 3.10+
- Git

Windows 可以单独运行：

```powershell
.\update_data.bat
```

Data Sync 不需要额外 Python 第三方包：Atlas 请求使用 Python 标准库。正常日常使用更推荐直接点网页中的 **更新游戏数据**，因为网页更新成功后会自动热重载。

## v5 Planner 功能保留

### Chaldea 羁绊 CSV

支持导入 Chaldea 羁绊详情 CSV：

- `svtId`：稳定 ID 匹配；
- `Rank` / `RankMax`：当前羁绊等级和上限；
- `Total`：累计羁绊；
- `Next`：距下一级；
- `Total(LvX)`：各羁绊等级累计阈值。

导入 CSV 会同步当前账号持有 Box，但保留人工设置的司机/老板、固定出场、手动优先度和培养目标。

### 长期培养

三种求解目标：

1. **单场真实羁绊最大**：只最大化本场游戏实际羁绊；
2. **长期 Box 均衡**：司机降培养权重、老板升权，结合精确羁绊缺口照顾平时不容易上场的低进度从者；
3. **优先收尾**：反向强调接近目标的从者，适合集中推到绊10/15。

“培养评分”只用于 Solver 排序，页面仍独立显示真实游戏羁绊，不会把权重伪装成实际收益。

## 冠位戴冠战

开启冠位模式后：

- 自备普通羁绊礼装 5 张，正常 Cost；
- 自家 Grand 额外报酬礼装 1 张，Solver 自动选择，0 Cost；
- 助战 Grand 两张羁绊礼装；
- 合计最多 8 个加成位；
- 必须选对应职阶；Cost 直接填游戏实际值。

## 绊10 / 绊15

- 绊10：当前未继续解锁，默认排除；
- 绊15：自身按满绊 0 收益，同时可作为「夢火の導き」辅助，每骑为其他可获羁绊的自家从者 +25%，多骑叠加；
- 固定满绊司机可以保持固定出场，由 Solver 继续优化剩余座位。

## Windows 构建 / 运行

首次或覆盖后端源码后：

```powershell
.\build_local.bat
```

运行：

```powershell
.\run_local.bat
```

如果官方 Go Proxy 不可达：

```powershell
go env -w GOPROXY=https://goproxy.cn,direct
```

## v7 验证重点

发布前至少确认：

- `node --check backend/static/fgo-response-import.js`
- `node --check backend/static/app.js`
- `node --check backend/static/picker.js`
- 用真实/脱敏 `toplogin` fixture 验证 Base64 / URL 编码 / 原始 JSON 三种输入得到一致 Box；礼装只采纳满破实例；
- Go 修改文件 `gofmt`
- HTML 无重复 `id`
- `ce.py` 的 machine parser 能从合成 `servantFriendshipUp` fixture 自动得到条件 trait + 满破 20%
- Data Sync 出错时旧 `servants.json / ces.json` 仍可继续启动；`source_meta.json` 只有整轮成功后才更新
- Windows Go 1.25.1 下 `build_local.bat` 通过后，再从页面实测一次“检查更新 → 更新游戏数据 → 热重载”。

## v6 Data Sync Hotfix 1 — Windows UTF-8

- Fixed `data.py` reading Chaldea Data JSON with the Windows locale default encoding (GBK/cp936), which could raise `UnicodeDecodeError` on UTF-8 mapping files such as `mappings/trait.json`.
- All Python text file reads/writes in the data generation path now use explicit UTF-8 (`utf-8-sig` for upstream JSON inputs, `utf-8` for generated outputs/state files).
- This hotfix only changes Python-side data generation. If v6 is already built, rebuilding the Go backend is not required; stop/restart the local server after overlaying the files and run **更新游戏数据** again.


## v6 Data Sync Hotfix2
- Atlas Academy is now optional for JP updates. Chaldea Data Git is the authoritative JP dataset source.
- If Atlas CN endpoints are blocked/unreachable, JP sync still succeeds; the previous CN snapshot is preserved and brand-new JP servants are conservatively treated as unavailable on CN until a later successful refresh.
- Data Sync status now distinguishes Chaldea/Git availability from optional Atlas availability.

## v6 Data Sync Hotfix3 — upstream schema hardening / rollback safety

Hotfix3 targets Windows/upstream-data failures observed after the Chaldea Data repository itself updated successfully.

- Git output is captured/condensed so the real Python failure is no longer buried by `git pull` diff output.
- Python child processes are forced to UTF-8 I/O on Windows.
- Chaldea mapping/event/servant parsing now tolerates missing regional fields, nullable translations, missing `ascensionAdd`, incomplete face variants, and auxiliary event-schema drift.
- A servant that fails the newest schema parser keeps its last known-good local record instead of silently disappearing from the catalog.
- Reviewed and previously discovered bond CEs are preserved if Chaldea temporarily reshuffles/dist-splits their source records.
- Data Sync is transactional: if either servant or CE generation fails, generated runtime data is restored to the pre-sync snapshot.
- Catalog shrink is treated as a broken source/parser contract and automatically rolled back. FGO historical servants/bond CEs should not disappear in bulk.
- Full generator logs are saved as `data/data_sync_data.log` and `data/data_sync_ce.log` for any future upstream schema break.

The Hotfix3 overlay also carries the last known-good 461-servant / 24-bond-CE baseline so a machine already affected by the failed v6 sync is repaired immediately on overwrite. Browser Box/planner settings live in localStorage and are not erased.

### v8 Local-only Hotfix1

- `build_local.bat` now runs `go mod tidy` after removing the legacy account/auth sources and before compiling.
- This closes the module metadata after removing JWT/bcrypt/SQLite dependencies, avoiding Go 1.25's `updates to go.mod needed` build refusal.
- The actual build is then run with `-mod=readonly`, so any remaining module inconsistency is treated as a real error instead of being silently rewritten during compilation.
