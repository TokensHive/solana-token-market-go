package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/TokensHive/solana-token-market-go/sdk/internal/pubkeyx"
	"github.com/TokensHive/solana-token-market-go/sdk/market"
	"github.com/gagliardetto/solana-go"
	"github.com/shopspring/decimal"
)

const (
	raydiumPoolsByMintURL = "https://api-v3.raydium.io/pools/info/mint"
	dexScreenerByMintURL  = "https://api.dexscreener.com/latest/dex/tokens/"
)

func discoverAPIFallback(ctx context.Context, req market.DiscoveryRequest) ([]*market.Pool, map[string]any, error) {
	if req.Mint.IsZero() {
		return nil, nil, nil
	}
	meta := map[string]any{}

	raydiumPools, raydiumErr := discoverRaydiumPoolsByMint(ctx, req.Mint)
	if raydiumErr != nil {
		meta["raydium_error"] = raydiumErr.Error()
	}

	dexPools, dexErr := discoverDexScreenerPoolsByMint(ctx, req.Mint)
	if dexErr != nil {
		meta["dexscreener_error"] = dexErr.Error()
	}

	merged := mergeAPIPools(raydiumPools, dexPools)
	if len(merged) > 0 {
		switch {
		case len(raydiumPools) > 0 && len(dexPools) > 0:
			meta["source"] = "raydium_api+dexscreener_api"
		case len(raydiumPools) > 0:
			meta["source"] = "raydium_api"
		default:
			meta["source"] = "dexscreener_api"
		}
		return merged, meta, nil
	}
	if raydiumErr != nil || dexErr != nil {
		return nil, meta, fmt.Errorf("api fallback could not resolve pools")
	}
	return nil, meta, nil
}

func mergeAPIPools(primary []*market.Pool, secondary []*market.Pool) []*market.Pool {
	if len(primary) == 0 {
		return secondary
	}
	merged := make([]*market.Pool, 0, len(primary)+len(secondary))
	byAddress := make(map[string]*market.Pool, len(primary)+len(secondary))
	for _, p := range primary {
		if p == nil || p.Address == "" {
			continue
		}
		merged = append(merged, p)
		byAddress[p.Address] = p
	}
	for _, p := range secondary {
		if p == nil || p.Address == "" {
			continue
		}
		if existing, ok := byAddress[p.Address]; ok {
			if existing.PriceOfTokenInSOL.IsZero() && !p.PriceOfTokenInSOL.IsZero() {
				existing.PriceOfTokenInSOL = p.PriceOfTokenInSOL
			}
			if existing.LiquidityInSOL.IsZero() && !p.LiquidityInSOL.IsZero() {
				existing.LiquidityInSOL = p.LiquidityInSOL
			}
			if existing.BaseReserve.IsZero() && !p.BaseReserve.IsZero() {
				existing.BaseReserve = p.BaseReserve
			}
			if existing.QuoteReserve.IsZero() && !p.QuoteReserve.IsZero() {
				existing.QuoteReserve = p.QuoteReserve
			}
			if existing.Metadata == nil {
				existing.Metadata = map[string]any{}
			}
			existing.Metadata["secondary_source"] = "dexscreener_api"
			continue
		}
		merged = append(merged, p)
		byAddress[p.Address] = p
	}
	return merged
}

type raydiumPoolsResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Count int                `json:"count"`
		Data  []raydiumPoolEntry `json:"data"`
	} `json:"data"`
}

type raydiumPoolEntry struct {
	Type        string `json:"type"`
	ProgramID   string `json:"programId"`
	ID          string `json:"id"`
	Price       any    `json:"price"`
	MintAmountA any    `json:"mintAmountA"`
	MintAmountB any    `json:"mintAmountB"`
	TVL         any    `json:"tvl"`
	MintA       struct {
		Address string `json:"address"`
	} `json:"mintA"`
	MintB struct {
		Address string `json:"address"`
	} `json:"mintB"`
}

func discoverRaydiumPoolsByMint(ctx context.Context, mint solana.PublicKey) ([]*market.Pool, error) {
	q := url.Values{}
	q.Set("mint1", mint.String())
	q.Set("poolType", "all")
	q.Set("poolSortField", "liquidity")
	q.Set("sortType", "desc")
	q.Set("pageSize", "30")
	q.Set("page", "1")
	endpoint := raydiumPoolsByMintURL + "?" + q.Encode()

	body, err := fetchJSON(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	var resp raydiumPoolsResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	if !resp.Success || len(resp.Data.Data) == 0 {
		return nil, nil
	}

	out := make([]*market.Pool, 0, len(resp.Data.Data))
	now := time.Now().UTC()
	for _, item := range resp.Data.Data {
		if strings.TrimSpace(item.ID) == "" {
			continue
		}
		baseMint := strings.TrimSpace(item.MintA.Address)
		quoteMint := strings.TrimSpace(item.MintB.Address)
		if baseMint == "" || quoteMint == "" {
			continue
		}
		price := decimalFromAny(item.Price)
		baseReserve := decimalFromAny(item.MintAmountA)
		quoteReserve := decimalFromAny(item.MintAmountB)
		liqSOL := decimal.Zero
		priceSOL := decimal.Zero
		if baseMint == mint.String() && pubkeyx.IsSOLMintString(quoteMint) {
			priceSOL = price
			liqSOL = quoteReserve
		} else if quoteMint == mint.String() && pubkeyx.IsSOLMintString(baseMint) {
			if !price.IsZero() {
				priceSOL = decimal.NewFromInt(1).Div(price)
			}
			liqSOL = baseReserve
		}
		protocol := market.ProtocolRaydiumCPMM
		marketType := market.MarketTypeConstantProduct
		if strings.EqualFold(item.Type, "Concentrated") {
			protocol = market.ProtocolRaydiumCLMM
			marketType = market.MarketTypeConcentratedLiquidity
		}
		out = append(out, &market.Pool{
			Address:           item.ID,
			Protocol:          protocol,
			MarketType:        marketType,
			BaseMint:          baseMint,
			QuoteMint:         quoteMint,
			BaseReserve:       baseReserve,
			QuoteReserve:      quoteReserve,
			PriceOfTokenInSOL: priceSOL,
			LiquidityInSOL:    liqSOL,
			LiquidityInQuote:  quoteReserve,
			IsActive:          true,
			IsVerified:        true,
			LastUpdatedAt:     now,
			Raw: market.RawPoolData{
				ProgramID: item.ProgramID,
				Extra: map[string]any{
					"source": "raydium_api",
					"tvl":    decimalFromAny(item.TVL).String(),
				},
			},
			Metadata: map[string]any{
				"source": "raydium_api",
			},
		})
	}
	return out, nil
}

type dexScreenerResponse struct {
	Pairs []dexScreenerPair `json:"pairs"`
}

type dexScreenerPair struct {
	DexID       string `json:"dexId"`
	PairAddress string `json:"pairAddress"`
	BaseToken   struct {
		Address string `json:"address"`
	} `json:"baseToken"`
	QuoteToken struct {
		Address string `json:"address"`
	} `json:"quoteToken"`
	PriceNative string `json:"priceNative"`
	PriceUSD    string `json:"priceUsd"`
	Liquidity   struct {
		Base  any `json:"base"`
		Quote any `json:"quote"`
		USD   any `json:"usd"`
	} `json:"liquidity"`
}

func discoverDexScreenerPoolsByMint(ctx context.Context, mint solana.PublicKey) ([]*market.Pool, error) {
	body, err := fetchJSON(ctx, dexScreenerByMintURL+mint.String())
	if err != nil {
		return nil, err
	}
	var resp dexScreenerResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	if len(resp.Pairs) == 0 {
		return nil, nil
	}
	out := make([]*market.Pool, 0, len(resp.Pairs))
	now := time.Now().UTC()
	for _, pair := range resp.Pairs {
		if strings.TrimSpace(pair.PairAddress) == "" {
			continue
		}
		baseMint := strings.TrimSpace(pair.BaseToken.Address)
		quoteMint := strings.TrimSpace(pair.QuoteToken.Address)
		if baseMint == "" || quoteMint == "" {
			continue
		}
		protocol, ok := protocolFromDexID(pair.DexID)
		if !ok {
			continue
		}
		marketType := marketTypeFromProtocol(protocol)
		priceNative, _ := decimal.NewFromString(strings.TrimSpace(pair.PriceNative))
		liqBase := decimalFromAny(pair.Liquidity.Base)
		liqQuote := decimalFromAny(pair.Liquidity.Quote)

		priceSOL := decimal.Zero
		liqSOL := decimal.Zero
		if baseMint == mint.String() && pubkeyx.IsSOLMintString(quoteMint) {
			priceSOL = priceNative
			liqSOL = liqQuote
		} else if quoteMint == mint.String() && pubkeyx.IsSOLMintString(baseMint) {
			if !priceNative.IsZero() {
				priceSOL = decimal.NewFromInt(1).Div(priceNative)
			}
			liqSOL = liqBase
		}

		out = append(out, &market.Pool{
			Address:           pair.PairAddress,
			Protocol:          protocol,
			MarketType:        marketType,
			BaseMint:          baseMint,
			QuoteMint:         quoteMint,
			BaseReserve:       liqBase,
			QuoteReserve:      liqQuote,
			PriceOfTokenInSOL: priceSOL,
			LiquidityInSOL:    liqSOL,
			LiquidityInQuote:  liqQuote,
			IsActive:          true,
			IsVerified:        true,
			LastUpdatedAt:     now,
			Metadata: map[string]any{
				"source":        "dexscreener_api",
				"price_usd":     pair.PriceUSD,
				"dex_id":        pair.DexID,
				"liquidity_usd": decimalFromAny(pair.Liquidity.USD).String(),
			},
		})
	}
	return out, nil
}

func protocolFromDexID(dexID string) (market.Protocol, bool) {
	switch strings.ToLower(strings.TrimSpace(dexID)) {
	case "raydium":
		return market.ProtocolRaydiumCPMM, true
	case "pumpswap":
		return market.ProtocolPumpswap, true
	case "pumpfun":
		return market.ProtocolPumpfun, true
	case "orca":
		return market.ProtocolOrcaWhirlpool, true
	case "meteora":
		return market.ProtocolMeteoraDAMM, true
	default:
		return "", false
	}
}

func marketTypeFromProtocol(protocol market.Protocol) market.MarketType {
	switch protocol {
	case market.ProtocolOrcaWhirlpool, market.ProtocolRaydiumCLMM, market.ProtocolMeteoraDLMM:
		return market.MarketTypeConcentratedLiquidity
	case market.ProtocolPumpfun:
		return market.MarketTypeBondingCurve
	case market.ProtocolRaydiumLaunchLab:
		return market.MarketTypeLaunchpad
	default:
		return market.MarketTypeConstantProduct
	}
}

func fetchJSON(ctx context.Context, endpoint string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "TokensHive-solana-token-market-go/1.0")

	client := &http.Client{Timeout: 12 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("status %d from %s", resp.StatusCode, endpoint)
	}
	return io.ReadAll(resp.Body)
}

func decimalFromAny(v any) decimal.Decimal {
	switch x := v.(type) {
	case nil:
		return decimal.Zero
	case float64:
		return decimal.NewFromFloat(x)
	case float32:
		return decimal.NewFromFloat(float64(x))
	case int:
		return decimal.NewFromInt(int64(x))
	case int64:
		return decimal.NewFromInt(x)
	case json.Number:
		d, err := decimal.NewFromString(x.String())
		if err == nil {
			return d
		}
		return decimal.Zero
	case string:
		x = strings.TrimSpace(x)
		if x == "" {
			return decimal.Zero
		}
		d, err := decimal.NewFromString(x)
		if err == nil {
			return d
		}
		f, ferr := strconv.ParseFloat(x, 64)
		if ferr == nil {
			return decimal.NewFromFloat(f)
		}
		return decimal.Zero
	default:
		s := strings.TrimSpace(fmt.Sprint(x))
		if s == "" {
			return decimal.Zero
		}
		d, err := decimal.NewFromString(s)
		if err == nil {
			return d
		}
	}
	return decimal.Zero
}
