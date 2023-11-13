package ton

import (
	"context"
	"fmt"
	"strconv"

	"github.com/xssnick/tonutils-go/liteclient"
	"github.com/xssnick/tonutils-go/ton"
)

func FindArchiveNode(ctx context.Context, cfg *liteclient.GlobalConfig) (ip, key string) {
	for _, liteSrv := range cfg.Liteservers {
		client := liteclient.NewConnectionPool()

		addr := fmt.Sprintf("%s:%d", intToIP4(liteSrv.IP), liteSrv.Port)
		if err := client.AddConnection(ctx, addr, liteSrv.ID.Key); err != nil {
			continue
		}

		api := ton.NewAPIClient(client)

		master, err := api.GetMasterchainInfo(ctx)
		if err != nil {
			continue
		}

		_, err = api.LookupBlock(ctx, master.Workchain, master.Shard, 3)
		if err != nil {
			continue
		}

		return addr, liteSrv.ID.Key
	}

	return "", ""
}

func intToIP4(ipInt int64) string {
	b0 := strconv.FormatInt((ipInt>>24)&0xff, 10)
	b1 := strconv.FormatInt((ipInt>>16)&0xff, 10)
	b2 := strconv.FormatInt((ipInt>>8)&0xff, 10)
	b3 := strconv.FormatInt(ipInt&0xff, 10)
	return b0 + "." + b1 + "." + b2 + "." + b3
}
