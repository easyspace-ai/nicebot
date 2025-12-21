# Dashboard Performance Optimization Guide

## 🔴 Critical Performance Issues Found

### Problem 1: `/api/statistics` Endpoint - Blocking API Calls
**Current Code:**
```python
for order in orders:
    if order.status in [OrderStatus.FILLED, OrderStatus.PARTIALLY_FILLED]:
        order_details = bot.order_manager.client.get_order(order.order_id)  # ⚠️ SLOW!
        size_matched = float(order_details.get("size_matched", 0))
```

**Issue:** Makes a synchronous API call for EVERY filled order in your history. If you have 100 orders, that's 100 sequential API calls (could take 10-50 seconds!).

### Problem 2: `/api/strategy-statistics` - Same Issue
Same blocking API calls for every order across all strategies.

### Problem 3: No Caching
Every dashboard refresh (every 5 seconds) recomputes everything from scratch.

---

## 🟢 Solutions (Ranked by Impact)

### Solution 1: Remove API Calls from Statistics Endpoints (Immediate - 90% improvement)

**Change:** Use cached `size_matched` data stored in OrderRecord instead of fetching live.

**Implementation:**

1. **Add `size_matched` field to OrderRecord model:**
```python
# models.py
class OrderRecord(BaseModel):
    # ... existing fields ...
    size_matched: float = 0.0  # Add this field
```

2. **Update `check_order_status()` to cache size_matched:**
```python
# order_manager.py
def check_order_status(self, order: OrderRecord) -> OrderRecord:
    order_details = self.client.get_order(order.order_id)
    if order_details:
        size_matched = float(order_details.get("size_matched", 0))
        order.size_matched = size_matched  # Cache it!
        # ... rest of logic
```

3. **Use cached data in dashboard:**
```python
# dashboard.py - /api/statistics
# REMOVE the API call, use cached data:
for order in orders:
    if order.status in [OrderStatus.FILLED, OrderStatus.PARTIALLY_FILLED]:
        size_matched = order.size_matched  # Use cached value!
        outcome_upper = order.outcome.strip().upper()
        if outcome_upper in ["YES", "UP"]:
            yes_filled += size_matched
        elif outcome_upper in ["NO", "DOWN"]:
            no_filled += size_matched
```

**Result:** 10-50 seconds → under 100ms

---

### Solution 2: Add Response Caching (Easy - 80% improvement)

**Change:** Cache expensive computations for 5-10 seconds.

**Implementation:**
```python
# dashboard.py
from datetime import datetime, timedelta
from typing import Optional

# Cache storage
_statistics_cache = {"data": None, "timestamp": None}
_strategy_cache = {"data": None, "timestamp": None}
CACHE_TTL_SECONDS = 5

@app.get("/api/statistics")
async def get_statistics():
    global _statistics_cache
    
    # Check cache
    now = datetime.now()
    if (_statistics_cache["data"] and _statistics_cache["timestamp"] and
        (now - _statistics_cache["timestamp"]).total_seconds() < CACHE_TTL_SECONDS):
        return _statistics_cache["data"]
    
    # Compute (expensive operation)
    result = _compute_statistics()
    
    # Update cache
    _statistics_cache = {"data": result, "timestamp": now}
    return result

def _compute_statistics():
    # Your existing logic here
    pass
```

---

### Solution 3: Lazy Load Statistics (UX - 100% perceived improvement)

**Change:** Load statistics only when user clicks a "Statistics" tab, not on every page load.

**Implementation:**
```javascript
// dashboard.html
// Only fetch statistics when user clicks the tab
document.getElementById('stats-tab').addEventListener('click', function() {
    fetch('/api/statistics')
        .then(response => response.json())
        .then(data => updateStatistics(data));
});
```

---

### Solution 4: Async API Calls with Concurrency (Advanced - 70% improvement)

**Change:** Make API calls concurrent instead of sequential.

**Implementation:**
```python
import asyncio
from concurrent.futures import ThreadPoolExecutor

executor = ThreadPoolExecutor(max_workers=10)

async def get_order_async(order_id: str):
    loop = asyncio.get_event_loop()
    return await loop.run_in_executor(
        executor, 
        bot.order_manager.client.get_order, 
        order_id
    )

@app.get("/api/statistics")
async def get_statistics():
    # Fetch all orders concurrently
    tasks = [get_order_async(order.order_id) for order in filled_orders]
    results = await asyncio.gather(*tasks, return_exceptions=True)
    # Process results...
```

---

### Solution 5: Reduce Auto-Refresh Frequency

**Change:** Dashboard refreshes every 5 seconds - too aggressive.

**Implementation:**
```javascript
// dashboard.html - change from:
setInterval(fetchAllData, 5000);  // Every 5 seconds

// to:
setInterval(fetchStatus, 5000);  // Lightweight status check every 5s
setInterval(fetchMarkets, 10000);  // Markets every 10s
setInterval(fetchOrders, 15000);  // Orders every 15s
// Statistics: only on demand (tab click)
```

---

## 🚀 Quick Fix (Copy-Paste Ready)

### Step 1: Add caching to statistics endpoint

```python
# dashboard.py - Add at top
_stats_cache = {"data": None, "ts": None}
_strategy_cache = {"data": None, "ts": None}
CACHE_TTL = 10  # 10 seconds

@app.get("/api/statistics")
async def get_statistics():
    global _stats_cache
    now = datetime.now()
    
    # Return cached if fresh
    if (_stats_cache["data"] and _stats_cache["ts"] and 
        (now - _stats_cache["ts"]).total_seconds() < CACHE_TTL):
        return _stats_cache["data"]
    
    try:
        bot = get_bot_instance()
        if not bot:
            return {"total_markets": 0, "successful_trades": 0, "unsuccessful_trades": 0}

        # Load order history
        try:
            with open("order_history.json", "r") as f:
                order_history_data = json.load(f)
                order_history = [OrderRecord(**order) for order in order_history_data]
        except FileNotFoundError:
            order_history = []

        markets = defaultdict(list)
        for order in order_history:
            markets[order.condition_id].append(order)

        total_markets = len(markets)
        successful_trades = 0
        unsuccessful_trades = 0

        for condition_id, orders in markets.items():
            yes_filled = 0.0
            no_filled = 0.0

            # USE CACHED DATA - NO API CALLS!
            for order in orders:
                if order.status in [OrderStatus.FILLED, OrderStatus.PARTIALLY_FILLED]:
                    outcome_upper = order.outcome.strip().upper()
                    # Use order.size as approximation (or size_matched if you added it to model)
                    if outcome_upper in ["YES", "UP"]:
                        yes_filled += order.size
                    elif outcome_upper in ["NO", "DOWN"]:
                        no_filled += order.size

            if yes_filled > 0 and no_filled > 0:
                successful_trades += 1
            else:
                unsuccessful_trades += 1

        result = {
            "total_markets": total_markets,
            "successful_trades": successful_trades,
            "unsuccessful_trades": unsuccessful_trades
        }
        
        # Cache result
        _stats_cache = {"data": result, "ts": now}
        return result

    except Exception as e:
        logger.error(f"Error getting statistics: {e}", exc_info=True)
        return {"total_markets": 0, "successful_trades": 0, "unsuccessful_trades": 0}
```

### Step 2: Same fix for strategy-statistics

Apply the same pattern (remove `bot.order_manager.client.get_order()` calls, add caching).

---

## 📊 Expected Results

| Fix | Load Time Before | Load Time After | Difficulty |
|-----|------------------|-----------------|------------|
| Remove API calls from stats | 10-50s | <100ms | Easy |
| Add caching | 10-50s | <50ms (cached) | Easy |
| Lazy load stats | Always slow | Only slow on-demand | Medium |
| Async API calls | 10-50s | 1-5s | Hard |
| Reduce refresh rate | N/A | Less server load | Easy |

---

## 🎯 Recommended Action Plan

1. **Immediate (5 minutes):** Add caching to `/api/statistics` and `/api/strategy-statistics`
2. **Short-term (15 minutes):** Remove all `get_order()` API calls from statistics endpoints
3. **Medium-term (30 minutes):** Add `size_matched` field to OrderRecord and cache it during status checks
4. **Optional:** Implement lazy loading for statistics tab

This will reduce your dashboard load time from **30+ seconds to under 1 second**.
