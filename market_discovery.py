"""Market discovery module for finding BTC 15-minute markets."""

import requests
from typing import List, Optional
from datetime import datetime, timedelta
from models import Market, Outcome
from logger import logger
from config import Config


class MarketDiscovery:
    """Discovers and tracks BTC 15-minute markets from Gamma API."""

    def __init__(self):
        self.base_url = Config.GAMMA_API_BASE_URL
        self.session = requests.Session()
        self.session.headers.update({
            "Content-Type": "application/json"
        })

    def discover_btc_15m_markets(self) -> List[Market]:
        """
        Discover upcoming and active BTC 15-minute markets.

        Uses proactive timestamp generation to find markets by slug pattern.
        Markets follow the format: btc-updown-15m-{unix_timestamp}
        where timestamp is the start of a 15-minute interval.

        Returns:
            List of Market objects for BTC 15m markets
        """
        try:
            btc_15m_markets = []

            # Generate upcoming 15-minute interval timestamps
            # Check next 48 intervals (12 hours forward)
            upcoming_timestamps = self._generate_15min_timestamps(48)

            logger.debug(f"Checking {len(upcoming_timestamps)} potential market slugs...")

            # Check each timestamp to see if market exists
            for timestamp in upcoming_timestamps:
                slug = f"btc-updown-15m-{timestamp}"

                # Try to fetch market by slug
                market_data = self._fetch_market_by_slug(slug)
                if market_data:
                    market = self._parse_market(market_data)
                    if market:
                        btc_15m_markets.append(market)
                        logger.debug(f"Found market: {slug}")

            # Sort by start time
            btc_15m_markets.sort(key=lambda m: m.start_timestamp)

            logger.info(f"Discovered {len(btc_15m_markets)} BTC 15m markets")
            return btc_15m_markets

        except Exception as e:
            logger.error(f"Error discovering markets: {e}", exc_info=True)
            return []

    def _generate_15min_timestamps(self, count: int) -> List[int]:
        """
        Generate upcoming 15-minute interval timestamps.

        Args:
            count: Number of intervals to generate

        Returns:
            List of unix timestamps at 15-minute intervals
        """
        from datetime import datetime, timedelta

        now = datetime.now()
        # Round down to nearest 15-minute mark
        current_15min = now.replace(second=0, microsecond=0)
        current_15min = current_15min.replace(minute=(current_15min.minute // 15) * 15)

        # Generate timestamps starting from next interval
        timestamps = []
        for i in range(count):
            future_time = current_15min + timedelta(minutes=15 * (i + 1))
            timestamps.append(int(future_time.timestamp()))

        return timestamps

    def _fetch_market_by_slug(self, slug: str) -> Optional[dict]:
        """
        Fetch a specific market by its slug.

        Args:
            slug: Market slug (e.g., btc-updown-15m-1766134800)

        Returns:
            Market data dict if found, None otherwise
        """
        try:
            url = f"{self.base_url}/events"
            params = {"slug": slug}

            response = self.session.get(url, params=params, timeout=5)
            response.raise_for_status()

            events = response.json()
            if events and len(events) > 0:
                return events[0]  # Return first result

            return None

        except Exception as e:
            logger.debug(f"Market {slug} not found: {e}")
            return None

    def _search_markets(self, query: str) -> List[dict]:
        """Search for markets using Gamma API."""
        try:
            # Try events endpoint first
            url = f"{self.base_url}/events"
            params = {
                "active": "true",
                "closed": "false",
                "limit": 100
            }

            response = self.session.get(url, params=params, timeout=10)
            response.raise_for_status()

            events = response.json()

            # Filter for BTC events and extract markets
            all_markets = []
            for event in events:
                if isinstance(event, dict):
                    # Check if event title contains BTC keywords
                    title = event.get("title", "").lower()
                    description = event.get("description", "").lower()

                    if "bitcoin" in title or "btc" in title or "bitcoin" in description:
                        # Get markets from this event
                        markets = event.get("markets", [])
                        all_markets.extend(markets)

            return all_markets

        except Exception as e:
            logger.error(f"Error searching markets: {e}")
            # Fallback: try direct markets endpoint
            return self._get_all_markets()

    def _get_all_markets(self) -> List[dict]:
        """Fallback method to get all markets."""
        try:
            url = f"{self.base_url}/markets"
            params = {"limit": 100, "active": "true"}

            response = self.session.get(url, params=params, timeout=10)
            response.raise_for_status()

            return response.json()
        except Exception as e:
            logger.error(f"Error getting all markets: {e}")
            return []

    def _parse_market(self, market_data: dict) -> Optional[Market]:
        """Parse market data from API response (event or market)."""
        try:
            # Handle both event and market data formats
            # Events have a "markets" array, individual markets have direct fields
            if "markets" in market_data and len(market_data["markets"]) > 0:
                # This is an event response, extract the first market
                actual_market = market_data["markets"][0]
                market_slug = market_data.get("slug")  # Use event slug
                question = actual_market.get("question") or market_data.get("title")
                condition_id = actual_market.get("conditionId")
            else:
                # This is a direct market response
                condition_id = market_data.get("conditionId") or market_data.get("condition_id")
                market_slug = market_data.get("slug") or market_data.get("market_slug")
                question = market_data.get("question") or market_data.get("title")
                actual_market = market_data

            if not all([condition_id, market_slug, question]):
                logger.debug(f"Missing required fields: condition_id={condition_id}, slug={market_slug}, question={question}")
                return None

            # Extract timestamps
            start_ts = None
            end_ts = None

            # Try to extract from slug first (most reliable for BTC markets)
            if "btc-updown-15m-" in market_slug:
                try:
                    ts_str = market_slug.split("btc-updown-15m-")[-1].split("-")[0]
                    start_ts = int(ts_str)
                    end_ts = start_ts + (15 * 60)  # Add 15 minutes
                except Exception as e:
                    logger.debug(f"Could not extract timestamp from slug: {e}")

            # Fallback: try to get from date fields
            if not start_ts:
                if "startDate" in actual_market:
                    start_ts = self._parse_iso_timestamp(actual_market["startDate"])
                elif "start_date" in market_data:
                    start_ts = self._parse_iso_timestamp(market_data["start_date"])

            if not end_ts:
                if "endDate" in actual_market:
                    end_ts = self._parse_iso_timestamp(actual_market["endDate"])
                elif "end_date" in market_data:
                    end_ts = self._parse_iso_timestamp(market_data["end_date"])

            if not start_ts or not end_ts:
                logger.debug(f"Could not extract timestamps for market: {market_slug}")
                return None

            # Extract outcomes and token IDs
            outcomes = []

            # Try clobTokenIds field first (most reliable)
            if "clobTokenIds" in actual_market:
                try:
                    import json
                    token_ids = json.loads(actual_market["clobTokenIds"]) if isinstance(actual_market["clobTokenIds"], str) else actual_market["clobTokenIds"]
                    outcomes_list = json.loads(actual_market["outcomes"]) if isinstance(actual_market.get("outcomes"), str) else actual_market.get("outcomes", ["Up", "Down"])

                    for i, token_id in enumerate(token_ids):
                        outcome_name = outcomes_list[i] if i < len(outcomes_list) else f"Outcome{i}"
                        outcomes.append(Outcome(
                            token_id=str(token_id),
                            outcome=outcome_name
                        ))
                except Exception as e:
                    logger.debug(f"Could not parse clobTokenIds: {e}")

            # Fallback: try tokens field
            if not outcomes:
                tokens = market_data.get("tokens", []) or actual_market.get("tokens", [])
                for token in tokens:
                    outcome = Outcome(
                        token_id=str(token.get("token_id", "")),
                        outcome=token.get("outcome", "")
                    )
                    outcomes.append(outcome)

            # Check if market is active or resolved
            is_active = market_data.get("active", False)
            is_resolved = market_data.get("closed", False) or market_data.get("resolved", False)

            return Market(
                condition_id=condition_id,
                market_slug=market_slug,
                question=question,
                start_timestamp=start_ts,
                end_timestamp=end_ts,
                outcomes=outcomes,
                is_active=is_active,
                is_resolved=is_resolved
            )

        except Exception as e:
            logger.error(f"Error parsing market data: {e}", exc_info=True)
            return None

    def _parse_iso_timestamp(self, iso_string: str) -> Optional[int]:
        """Parse ISO timestamp string to unix timestamp."""
        try:
            dt = datetime.fromisoformat(iso_string.replace("Z", "+00:00"))
            return int(dt.timestamp())
        except:
            return None

    def _is_btc_15m_market(self, market: Market) -> bool:
        """Check if market is a BTC 15-minute market."""
        # Check slug pattern
        if "btc-updown-15m" in market.market_slug.lower():
            return True

        # Check question content
        question_lower = market.question.lower()
        if all(keyword in question_lower for keyword in ["bitcoin", "15", "minute"]):
            # Verify it's actually a 15-minute window
            duration = market.end_timestamp - market.start_timestamp
            if 850 <= duration <= 950:  # 15 minutes ± some tolerance
                return True

        return False

    def get_market_orderbook(self, token_id: str) -> Optional[dict]:
        """Get orderbook for a specific token."""
        try:
            # This would typically use the CLOB API
            # For now, return None as we'll get this from the CLOB client
            return None
        except Exception as e:
            logger.error(f"Error getting orderbook for {token_id}: {e}")
            return None
