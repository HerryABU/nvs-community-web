# API Key 爬虫接入指南（Python requests）

> 本仓库 Markdown 为源码级参考；**应用内已内置同名页面**，随二进制一同部署，可在运行时打开：
> 用户菜单 → 「API Key 爬虫指南」或直接访问 `/api-spider`（个人设置 → API Key 管理卡片内也有入口）。

> 适用于需要批量、自动化读取 NVS 公开数据的脚本/爬虫/第三方客户端。
> 携带 API Key 后身份从「匿名」升级为「reader/author/admin」等角色，配额远高于匿名通道，且不会被误判为爬虫而降级。

## 1. 申请 API Key

1. 登录站点后进入 **个人设置 → API Key 管理**。
2. 填写备注名与自设配额，点击「创建 API Key」。
3. **完整 Key 仅展示一次**，请立即复制保存；后台只存 SHA-256 哈希，丢失后只能重新生成。

> 管理员可在 **管理后台 → 限流与 API Key** 中查看全站 Key 的累计调用次数、最近使用时间、状态等。

## 2. 调用方式

所有公开 API 均在请求头中携带 `X-API-Key`：

```python
import requests

BASE_URL = "https://your-nvs.example.com"  # 如反向代理前缀为 /go-nvs，则写 https://example.com/go-nvs
API_KEY = "nvs_rdr_3f2a1bxxxxxxxxxxxxxxxxxxxxxxxx"

headers = {
    "X-API-Key": API_KEY,
    "User-Agent": "MyNVSReader/1.0 (contact@example.com)",  # 建议写真实联系方式，便于被联系
    "Accept": "application/json",
}

r = requests.get(f"{BASE_URL}/api/novels?page=1&page_size=20", headers=headers)
print(r.status_code, r.json())
```

返回结构示例：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "list": [...],
    "total": 128,
    "page": 1,
    "page_size": 20
  }
}
```

## 3. 常见公开接口

| 接口 | 说明 | 示例 |
|------|------|------|
| `GET /api/novels` | 作品列表 | `?page=1&page_size=20&search=关键词&sort_by=featured` |
| `GET /api/novels/:id` | 作品详情 | `/api/novels/42` |
| `GET /api/novels/:id/chapters` | 章节列表 | `/api/novels/42/chapters` |
| `GET /api/novels/:id/chapters/:num` | 章节正文 | `/api/novels/42/chapters/1` |
| `GET /api/categories` | 分类列表 | 无参数 |
| `GET /api/novels/:id/forum` | 作品论坛帖子 | `?page=1&page_size=20` |

> 若站点挂在反向代理前缀下（如 `/go-nvs`），请把前缀加在 `/api` 之前，例如 `https://example.com/go-nvs/api/novels`。

## 4. 分页遍历示例

```python
import time

PAGE_SIZE = 50

for page in range(1, 1000):
    url = f"{BASE_URL}/api/novels?page={page}&page_size={PAGE_SIZE}&sort_by=updated_at"
    r = requests.get(url, headers=headers)
    r.raise_for_status()
    data = r.json().get("data", {})
    items = data.get("list", [])
    if not items:
        break

    for novel in items:
        process_novel(novel)  # 你的业务逻辑

    # 到达最后一页
    if page * PAGE_SIZE >= data.get("total", 0):
        break

    time.sleep(0.3)  # 即使在配额内，也建议礼貌访问
```

## 5. 限流与重试

- 无 Key 的匿名访问配额很低，且被识别为爬虫后会进一步降到更低配额。
- 携带 Key 后按 Key 维度限流；到达上限会收到 `429 Too Many Requests`。
- 建议实现指数退避重试：

```python
import requests
from requests.adapters import HTTPAdapter, Retry

session = requests.Session()
retries = Retry(total=5, backoff_factor=1, status_forcelist=[429, 500, 502, 503, 504])
session.mount("https://", HTTPAdapter(max_retries=retries))
session.mount("http://", HTTPAdapter(max_retries=retries))

# 之后用 session.get(...)
```

## 6. 查看调用次数

- **用户侧**：个人设置 → API Key 管理 → 表格中的「调用次数」。
- **管理侧**：管理后台 → 限流与 API Key → 「全站用户 Key 一览」中的「调用次数」。

调用次数在每次携带有效 Key 的请求被成功处理后自动累计；重新生成 Key 后次数归零。

## 7. 注意事项

1. **不要泄露 Key**：不要把 Key 写到前端代码或公开仓库中。如怀疑泄露，立即在后台「停用」或「删除」该 Key。
2. **IP 白名单**：可在创建 Key 时限制来源 IP/CIDR，进一步降低被盗用风险。
3. **过期时间**：可为 Key 设置过期时间，到期后自动失效。
4. **User-Agent**：建议填写可识别的 `User-Agent` 与联系方式，避免被管理员误判为恶意爬虫。
