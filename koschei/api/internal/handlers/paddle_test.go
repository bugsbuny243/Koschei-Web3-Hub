package handlers

import "testing"

func TestPaddleConfiguredPriceIDUsesActiveBillingPriceIDs(t *testing.T) {
	t.Setenv("PADDLE_STARTER_PRICE_ID", "")
	t.Setenv("PADDLE_BUILDER_PRICE_ID", "")
	t.Setenv("PADDLE_STUDIO_PRICE_ID", "")

	cases := map[string]string{
		"starter": "pri_01ktvdpq7fr4gknbr4wtavrfrb",
		"builder": "pri_01ktvhhg4z9j3wjrts1kzswhm",
		"studio":  "pri_01ktvhqya0rj2y8850yvd37d92",
	}
	productNames := map[string]string{
		"starter": "Koschei Starter",
		"builder": "Koschei Professional",
		"studio":  "Koschei Enterprise",
	}
	for tier, want := range cases {
		if got := paddleProductName(tier); got != productNames[tier] {
			t.Fatalf("paddleProductName(%q) = %q, want %q", tier, got, productNames[tier])
		}
		if got := paddleConfiguredPriceID(tier); got != want {
			t.Fatalf("paddleConfiguredPriceID(%q) = %q, want %q", tier, got, want)
		}
		if got := paddleTransactionItem(want, tier)["price_id"]; got != want {
			t.Fatalf("paddleTransactionItem(%q, %q) price_id = %q, want %q", want, tier, got, want)
		}
	}
}

func TestPaddleConfiguredPriceIDAllowsExistingEnvOverride(t *testing.T) {
	t.Setenv("PADDLE_STARTER_PRICE_ID", "pri_custom_starter")

	if got := paddleConfiguredPriceID("starter"); got != "pri_custom_starter" {
		t.Fatalf("paddleConfiguredPriceID starter override = %q, want pri_custom_starter", got)
	}
}

func TestSubscriptionProductAndTierMapsFromPriceID(t *testing.T) {
	t.Setenv("PADDLE_STARTER_PRICE_ID", "")
	t.Setenv("PADDLE_BUILDER_PRICE_ID", "")
	t.Setenv("PADDLE_STUDIO_PRICE_ID", "")

	sub := paddleSubscriptionData{
		Items: []struct {
			Price struct {
				ID        string `json:"id"`
				ProductID string `json:"product_id"`
			} `json:"price"`
		}{
			{
				Price: struct {
					ID        string `json:"id"`
					ProductID string `json:"product_id"`
				}{ID: "pri_01ktvhhg4z9j3wjrts1kzswhm", ProductID: "pro_active_professional"},
			},
		},
	}

	productID, tier := subscriptionProductAndTier(sub)
	if productID != "pro_active_professional" {
		t.Fatalf("subscriptionProductAndTier productID = %q, want pro_active_professional", productID)
	}
	if tier != "builder" {
		t.Fatalf("subscriptionProductAndTier tier = %q, want builder", tier)
	}
}
