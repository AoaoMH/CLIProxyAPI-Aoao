# 使用记录完整请求体支持

## 修改说明

为了支持在管理界面中显示完整的请求体和响应体内容，对后端进行了以下修改：

## 1. 列表接口返回完整字段

**文件**: `internal/usagerecord/store.go`

**修改**: `List` 方法现在返回完整的 `request_body` 和 `response_body` 字段

**之前**:
```sql
SELECT id, request_id, timestamp, ip, api_key, api_key_masked, model, provider,
    is_streaming, input_tokens, output_tokens, total_tokens,
    duration_ms, status_code, success, request_url, request_method
FROM usage_records
```

**之后**:
```sql
SELECT id, request_id, timestamp, ip, api_key, api_key_masked, model, provider,
    is_streaming, input_tokens, output_tokens, total_tokens,
    duration_ms, status_code, success, request_url, request_method,
    request_headers, request_body, response_headers, response_body
FROM usage_records
```

## 2. 移除字段截断限制

**文件**: `internal/usagerecord/plugin.go`

**修改**: 移除了 `truncateBody` 函数调用，完整存储请求体和响应体

**之前**:
```go
requestBody = truncateBody(string(bodyBytes), 50000) // 截断到 50KB
```

**之后**:
```go
requestBody = string(bodyBytes) // 完整存储，不截断
```

## 3. 完善字段解析

**文件**: `internal/usagerecord/store.go`

**修改**: 在 `List` 方法中添加了对 `request_headers` 和 `response_headers` 的 JSON 解析

```go
// Parse headers JSON
if err := json.Unmarshal([]byte(reqHeadersJSON), &r.RequestHeaders); err != nil {
    r.RequestHeaders = make(map[string]string)
}
if err := json.Unmarshal([]byte(respHeadersJSON), &r.ResponseHeaders); err != nil {
    r.ResponseHeaders = make(map[string]string)
}
```

## 影响和好处

### ✅ 好处
1. **完整数据显示**: 前端可以显示完整的请求体和响应体，不再被截断
2. **统一接口**: 列表接口和详情接口返回相同的完整数据
3. **更好的调试体验**: 开发者可以看到完整的 API 调用内容
4. **支持长提示词**: 特别适合 AI 模型的长系统提示词场景

### ⚠️ 注意事项
1. **数据库大小**: 完整存储会增加数据库大小
2. **网络传输**: 列表接口的响应体会变大
3. **内存使用**: 加载大量记录时会占用更多内存

### 🔧 性能优化建议
1. **分页大小**: 建议将默认分页大小从 20 调整为 10，减少单次传输数据量
2. **索引优化**: 确保数据库索引正确设置
3. **缓存策略**: 考虑在前端添加适当的缓存机制

## API 接口变化

### `/management/usage-records` (列表接口)

**之前**: 只返回基本字段，不包含 `request_body` 和 `response_body`

**之后**: 返回完整字段，包含完整的 `request_body` 和 `response_body`

### `/management/usage-records/:id` (详情接口)

**无变化**: 继续返回完整字段

## 前端适配

前端的 `RecordDetailDrawerNew` 组件不需要修改，因为它已经正确处理了完整的数据字段。现在列表接口直接返回完整数据，前端可以立即显示，无需额外的详情接口调用。

## 测试验证

1. 启动后端服务
2. 发送包含长请求体的 API 请求
3. 在管理界面查看使用记录列表
4. 确认请求体完整显示，无 `[truncated]` 标记

## 回滚方案

如果需要回滚到截断模式，可以：

1. 恢复 `List` 方法的 SELECT 查询，移除 `request_body` 和 `response_body` 字段
2. 恢复 `plugin.go` 中的 `truncateBody` 调用
3. 前端继续使用详情接口获取完整数据