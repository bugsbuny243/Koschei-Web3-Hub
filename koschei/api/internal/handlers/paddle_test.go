package handlers

import "testing"

func TestPaddleConfiguredPriceIDUsesActiveBillingPriceIDs(t *testing.T) {
	t.Setenv("PADDLE_STARTER_PRICE_ID", "")
	t.Setenv("PADDLE_PROFESSIONAL_PRICE_ID", "pri_professional")
	t.Setenv("PADDLE_ENTERPRISE_PRICE_ID", "pri_enterprise")
	t.Setenv("PADDLE_BUILDER_PRICE_ID", "")
	t.Setenv("PADDLE_STUDIO_PRICE_ID", "")

	cases := map[string]string{
		"starter":      "",
		"professional": "pri_professional",
		"enterprise":   "pri_enterprise",
	}
	productNames := map[string]string{
		"starter":      "Koschei Starter",
		"professional": "Koschei Professional",
		"enterprise":   "Koschei Enterprise",
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
	t.Setenv("PADDLE_BUILDER_PRICE_ID", "pri_legacy_builder")

	if got := paddleConfiguredPriceID("starter"); got != "pri_custom_starter" {
		t.Fatalf("paddleConfiguredPriceID starter override = %q, want pri_custom_starter", got)
	}
	if got := paddleConfiguredPriceID("builder"); got != "pri_legacy_builder" {
		t.Fatalf("paddleConfiguredPriceID legacy builder override = %q, want pri_legacy_builder", got)
	}
}

func TestSubscriptionProductAndTierMapsFromPriceID(t *testing.T) {
	t.Setenv("PADDLE_STARTER_PRICE_ID", "")
	t.Setenv("PADDLE_PROFESSIONAL_PRICE_ID", "pri_professional")
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
				}{ID: "pri_professional", ProductID: "pro_active_professional"},
			},
		},
	}

	productID, tier := subscriptionProductAndTier(sub)
	if productID != "pro_active_professional" {
		t.Fatalf("subscriptionProductAndTier productID = %q, want pro_active_professional", productID)
	}
	if tier != "professional" {
		t.Fatalf("subscriptionProductAndTier tier = %q, want professional", tier)
	}
}
