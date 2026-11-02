package conf_test

import (
	"encoding/json"
	"testing"

	"github.com/xtls/xray-core/app/log"
	"github.com/xtls/xray-core/infra/conf"
	"google.golang.org/protobuf/proto"
)

func TestTrafficLogConfig(t *testing.T) {
	var c conf.LogConfig
	if err := json.Unmarshal([]byte(`{"loglevel":"none","traffic":"/var/log/xray/traffic.csv"}`), &c); err != nil {
		t.Fatal(err)
	}
	data, err := proto.Marshal(c.Build())
	if err != nil {
		t.Fatal(err)
	}
	var decoded log.Config
	if err := proto.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.TrafficLogPath != "/var/log/xray/traffic.csv" {
		t.Fatal(decoded.TrafficLogPath)
	}
}
