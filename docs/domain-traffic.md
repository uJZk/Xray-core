# 按目标域名统计流量

配置 `"stats": {}` 并启用 API 的 `StatsService` 后，域名计数自动启用，无需配置用户或 email：

```json
{
  "stats": {},
  "api": {"tag": "api", "services": ["StatsService"]}
}
```

这只是相关配置片段，API 入口和路由仍需按现有部署配置。

查询：

```sh
xray api statsquery --server=127.0.0.1:8080 -pattern 'domain>>>'
```

计数名称为 `domain>>>example.com>>>traffic>>>uplink` 和
`domain>>>example.com>>>traffic>>>downlink`，单位为字节。域名统一小写并去掉末尾的点；不合并子域名。多个用户的流量合并统计。

统计 dispatcher 嗅探后的目标域名，包括缓存的首包。客户端提供域名时无需嗅探；目标是 IP 时，需启用 sniffing 并配置相应的 destOverride（可使用 routeOnly）以识别域名，未识别出域名时按目标 IP 计数。IPv4 和 IPv6 均支持，同一 IP 的不同端口合并统计。为保持查询方式不变，IP 也使用 `domain>>>` 前缀，例如 `domain>>>192.0.2.1>>>traffic>>>uplink`、`domain>>>2001:db8::1>>>traffic>>>downlink`。

采用 Xray 原有 payload 统计口径：上行计读取的 payload，下行计交给 writer 的 payload。它不是网卡流量或远端确认接收的字节数；不含入口 TLS、VLESS 协议头和 Vision padding。Vision 下行 splice 会直接补记域名和已有用户计数，保留 splice。

VLESS fallback 不经过 dispatcher，也没有代理目标域名，故不计入域名统计；入口 SNI 不用作目标域名。

计数保存在内存，重启清零，也可通过 Stats API reset。每个不同域名或 IP 创建两个计数，长期运行时需注意域名数量带来的内存增长。

参考开源实现：

- [XTLS/Xray-core SizeStatWriter](https://github.com/XTLS/Xray-core/blob/main/app/dispatcher/stats.go)：复用其 Stats counter / payload 统计方式。
- [XTLS/Xray-core CopyRawConnIfExist](https://github.com/XTLS/Xray-core/blob/main/proxy/proxy.go)：沿用 splice 完成后补记字节数的方式，并支持嵌套域名／用户 writer。

## 直接追加流量 CSV

在 `log` 中设置 `traffic` 即可，无需启用 `stats` 或 StatsService：

```json
{
  "log": {
    "loglevel": "warning",
    "dnsLog": false,
    "traffic": "/var/log/xray/traffic.csv"
  }
}
```

表头固定为：

```csv
interval_start,interval_end,target,target_ip,uplink_bytes,downlink_bytes
```

- 每 60 秒追加一次该区间的增量，仅输出有变化的 `(target, target_ip)`；关闭日志时补写剩余增量。
- 时间使用 UTC RFC 3339（可带小数秒），两列分别表示本次统计区间的起止时间，不是连接起止时间。
- `target` 是嗅探后的域名或纯 IP，沿用上述规范化规则。`target_ip` 是直接 freedom TCP 出站实际连接的 IP；同一域名的不同 IP 分行累计。纯 IP 目标直接填入该 IP。域名经其他代理出站、拨号链或 UDP 等无法在当前挂点确认最终 IP 时，此列为空，不会额外 DNS 查询或填入代理服务器 IP。
- `target_sld` 不输出。多个用户、多个端口合并统计，fallback 排除。
- CSV 使用独立增量累计，Stats API 的查询或 reset 不影响它。流量口径仍为转发 payload，包含目标 HTTPS 自身的 TLS 数据，不含入口 TLS、VLESS 头和 Vision padding；Vision splice 保留并计数。
- 文件始终追加，重启续写，只有空文件才写表头。已有文件表头不匹配或末尾缺少换行时，启动报错而不覆盖历史。
- 后台批量写入并执行文件同步，转发过程仅更新内存。运行中写入失败会记录 warning，并保留增量到下次重试；关闭时仍无法写入则返回错误。健康磁盘下，强制结束进程可能丢失最后一个未写入周期；这不等同于逐字节持久化。
- 每批重新打开路径，支持 rename/create 式日志轮转；新文件仍须允许 Xray 写入。每个 CSV 路径只供一个 Xray 进程使用。
- 未配置 `traffic` 或设置为空字符串时关闭 CSV。`loglevel` 只控制普通错误日志，`maskAddress` 不处理 CSV 中的目标和 IP。

Docker 应将 `/var/log/xray` 挂载为持久目录，并让镜像运行用户 UID/GID `65532:65532` 可写。配置目录 `/usr/local/etc/xray` 与日志目录分开；新 CSV 默认权限为 `0600`。也可使用自定义路径，但父目录必须存在且可写。文件会持续增长，需按部署需求轮转或归档。

CSV 是增量明细；查看排行时再按 `target` 汇总上下行之和并排序，不重写历史文件。
