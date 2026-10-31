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
