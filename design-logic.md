明白了，你的 couswee 有 Go 后端，那我们可以完全实现“前端 SvelteKit 交互 + 后端 Go 写入 prodex profile / auth.json + 支持 CLI 使用”的方案。下面我给你做一份**完整方案文档**，结构化、可直接落地：

---

# Couswee 添加 Codex 帐号方案

## 1️⃣ 总体架构

```
+------------------+        REST / WebSocket       +------------------+
| SvelteKit 前端   | <--------------------------> | Go 后端 API      |
| (浏览器 UI)      |                              | (业务 & 文件写入)|
+------------------+                              +------------------+
         |                                                |
         | OAuth / 设备码流程                              | 生成 / 写入 prodex profile
         |                                                | 调用 prodex CLI 检查登录 / quota
         v                                                v
      用户交互界面                                 ~/.prodex/profiles/<profile>/auth.json
```

核心思路：

* 前端负责 UI、流程提示、收集用户输入（OAuth 授权 / 设备码）
* 后端负责操作文件系统、写入 `auth.json`，调用 CLI 命令（`prodex login` / `prodex use`）
* CLI / prodex 运行时直接使用对应 profile 的 `auth.json`

---

## 2️⃣ 用户添加流程

### A. **网页登录登录**

1. **前端操作**

   * 用户点击“添加账号 → Web 登录”按钮
   * 前端向后端发起 `GET /codex/oauth/init` 请求
   * 后端返回一个 **OAuth 授权 URL**，前端打开新窗口跳转到 Codex 授权页

2. **用户授权**

   * Codex OAuth 完成后，重定向到你的前端回调 URL，携带授权 code
   * 前端收到 code，通过 `POST /codex/oauth/callback` 发送给后端

3. **后端操作**

   * Go 后端用 code 调用 Codex OAuth token 接口，获取 access_token / refresh_token
   * 生成对应 prodex profile 名字（如 `user-<uuid>`）
   * 创建目录：

     ```
     ~/.prodex/profiles/<profile_name>/
     ```
   * 写入 `auth.json`：

     ```json
     {
       "access_token": "...",
       "refresh_token": "...",
       "expiry": 1700000000
     }
     ```
   * 可选：调用 `prodex current --profile <profile_name>` 或 `prodex run` 检查登录有效性

4. **前端更新**

   * 返回 profile 信息给前端
   * 前端显示“已添加账号”，支持切换 / 删除 / 查看 quota

---

### B. **设备码登录**

1. **前端操作**

   * 用户点击“添加账号 → 设备码登录”
   * 前端请求后端接口 `POST /codex/device/start`
   * 后端调用：

     ```bash
     codex login --device --profile <temp_profile>
     ```

     * CLI 返回 device_code / verification URL
   * 后端返回给前端：

     ```json
     {
       "device_code": "XXXX-XXXX",
       "verification_url": "https://codex.dev/device"
     }
     ```

2. **用户操作**

   * 前端显示设备码和 URL
   * 用户打开浏览器，输入 device_code 登录

3. **后端轮询**

   * 后端定期调用 `codex login --device --poll <device_code>` 检查是否完成
   * 完成后，将 token 写入 prodex profile：

     ```
     ~/.prodex/profiles/<profile_name>/auth.json
     ```

4. **前端更新**

   * 后端返回登录成功状态
   * 前端显示“已登录账号”，可切换 / 删除 / 查看 quota

---

## 3️⃣ API 设计示例（Go 后端）

| 方法   | 路径                                | 描述                                   |
| ------ | ----------------------------------- | -------------------------------------- |
| GET    | `/codex/oauth/init`                 | 返回 OAuth URL                         |
| POST   | `/codex/oauth/callback`             | 使用 code 获取 token 并写入 profile    |
| POST   | `/codex/device/start`               | 启动设备码登录，返回 device_code + URL |
| GET    | `/codex/device/poll?profile=<name>` | 轮询设备码登录状态                     |
| POST   | `/codex/profile/switch`             | 切换当前 active profile                |
| GET    | `/codex/profile/list`               | 返回所有 profile + quota / 状态        |
| DELETE | `/codex/profile/<name>`             | 删除指定 profile                       |

---

## 4️⃣ SvelteKit 前端界面设计

1. **账号列表页**

   * 显示当前 profile、quota、状态
   * 按钮：添加账号、删除账号、切换账号

2. **添加账号弹窗**

   * 两个选项：**网页登录** / **设备码登录**
   * **网页登录**：

     * 点击后跳转到 OAuth URL
     * 回调完成后显示“已登录”
   * **设备码登录**：

     * 显示 `verification_url + device_code`
     * 状态轮询显示“等待登录 / 登录成功”

3. **切换账号**

   * 点击某个 profile → 调用 `POST /codex/profile/switch`
   * 更新界面当前 active profile

---

## 5️⃣ Prodex 配置与文件结构

```
~/.prodex/
└─ profiles/
   ├─ main/          # profile_name = main
   │   └─ auth.json
   ├─ user-1/
   │   └─ auth.json
   └─ user-2/
       └─ auth.json
```

* 每个 profile 独立 `auth.json`
* 后端 Go 操作写入文件，前端只显示状态

---

## 6️⃣ 核心注意点

1. **安全性**

   * token 写入 profile 时，文件权限设置为 600
   * 前端不要直接存 token，只显示必要信息（用户名 / quota）

2. **流程一致性**

   * OAuth 与设备码登录最终都生成 prodex profile + auth.json
   * CLI / prodex run 时只需指定 `--profile <name>` 即可使用该 token

3. **状态管理**

   * 后端可以维护 profile 状态表（active / last-used / quota）
   * 前端定时轮询或 WebSocket 实时更新

---

✅ 总结：

* 两种登录方式本质都转换为**写入 prodex profile 对应的 `auth.json`**
* SvelteKit 前端负责 UI + 用户交互
* Go 后端负责生成 profile、写 token、调用 prodex CLI 确认状态
* 完全支持 CLI 使用，profile 切换、轮询、删除都可落地

---

如果你愿意，我可以帮你直接画一张 **完整流程图**，把 OAuth / 设备码 / profile 写入 / CLI 调用的流程一览，方便前端和后端开发同时参照。

你想让我画吗？