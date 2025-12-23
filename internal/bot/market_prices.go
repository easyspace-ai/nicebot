package bot

import (
	"context"

	"limitorderbot/internal/models"
)

func (b *Bot) fillMarketPrices(ctx context.Context, markets []models.Market) []models.Market {
	for i := range markets {
		m := markets[i]
		for j := range m.Outcomes {
			tok := m.Outcomes[j].TokenID
			if tok == "" {
				continue
			}
			book, err := b.clob.GetOrderBook(ctx, tok)
			if err != nil {
				continue
			}
			bid := bestBidFromBook(book)
			ask := bestAskFromBook(book)
			if bid > 0 {
				m.Outcomes[j].BestBid = &bid
			}
			if ask > 0 {
				m.Outcomes[j].BestAsk = &ask
			}
			if bid > 0 && ask > 0 {
				mid := (bid + ask) / 2
				m.Outcomes[j].Price = &mid
			}
		}
		markets[i] = m
	}
	return markets
}
