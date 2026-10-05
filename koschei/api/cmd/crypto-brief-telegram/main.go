// Configure a dedicated Crypto Brief bot from deployment-managed environment secrets.
// Run only in that trusted runtime; this command never prints credentials or provider bodies.
package main

import (
	"context"
	"fmt"
	"os"

	"koschei/api/internal/cryptobrief"
)

func main() {
	if err := cryptobrief.ConfigureTelegramWebhook(context.Background(), cryptobrief.ConfigFromEnv(), nil); err != nil {
		// ConfigureTelegramWebhook returns constant, credential-free errors only.
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("Crypto Brief Telegram webhook configuration verified. Test a consenting customer connection before marking delivery live.")
}
