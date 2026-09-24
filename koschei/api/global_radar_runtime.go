package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	koscheiclickhouse "koschei/api/internal/clickhouse"
	apihttp "koschei/api/internal/http"
)

const globalRadarClickHouseStartupTimeout = 10 * time.Second

func buildGlobalRadarSnapshotSink(parent context.Context) (apihttp.GlobalRadarSnapshotSink, error) {
	if strings.TrimSpace(os.Getenv("KOSCHEI_GLOBAL_RADAR_CLICKHOUSE_ENABLED")) != "1" {
		return nil, nil
	}

	client, err := koscheiclickhouse.NewFromEnv()
	if err != nil {
		return nil, fmt.Errorf("create Global Radar ClickHouse client: %w", err)
	}

	ctx, cancel := context.WithTimeout(parent, globalRadarClickHouseStartupTimeout)
	defer cancel()
	if err := client.VerifyGlobalRadarGraphSchema(ctx); err != nil {
		return nil, fmt.Errorf("verify Global Radar ClickHouse schema: %w", err)
	}
	return client, nil
}
