# File Reference Guide

Complete reference of all files in the Polymarket Limit Order Bot project.

## 📁 Directory Structure

```
limitorderbot-claude/
├── Core Application (Python)
├── Web Interface (HTML/JS)
├── Configuration Files
├── Documentation
├── Utility Scripts
└── Development Files
```

## 🐍 Core Application Files

### [main.py](main.py)
**Purpose:** Main entry point for the application
- CLI argument parsing (--mode, --check-config)
- Configuration validation
- Bot startup orchestration
- Mode selection (bot/dashboard/both)

**Usage:**
```bash
python main.py                    # Start bot + dashboard
python main.py --mode bot         # Bot only
python main.py --check-config     # Validate config
```

---

### [bot.py](bot.py)
**Purpose:** Core bot orchestration and main loop
- Market discovery coordination
- Order placement timing
- Active order monitoring
- State management
- Market cleanup
- Thread-safe operations

**Key Classes:**
- `PolymarketBot` - Main bot controller

**Key Functions:**
- `run_once()` - Single iteration of bot loop
- `_process_market()` - Handle individual market
- `_check_active_orders()` - Monitor order fills

---

### [config.py](config.py)
**Purpose:** Configuration management
- Load settings from .env file
- Type conversion and defaults
- Configuration validation
- Environment variable handling

**Key Class:**
- `Config` - Global configuration singleton

**Key Settings:**
- `PRIVATE_KEY` - Wallet authentication
- `ORDER_SIZE_USD` - Trading parameters
- `SPREAD_OFFSET` - Pricing strategy
- `DASHBOARD_PORT` - Web UI settings

---

### [logger.py](logger.py)
**Purpose:** Logging configuration
- Dual-handler setup (file + console)
- Structured log formatting
- Log level management
- Timestamp formatting

**Key Function:**
- `setup_logger()` - Initialize logger

**Output:**
- File: `bot.log` (DEBUG level)
- Console: stdout (INFO level)

---

### [models.py](models.py)
**Purpose:** Data models using Pydantic
- Type-safe data structures
- Data validation
- Computed properties
- Enumerations

**Key Models:**
- `Market` - Market data with start/end times
- `OrderRecord` - Order tracking
- `Outcome` - Market outcome (Yes/No)
- `BotState` - Current bot state
- `OrderStatus` - Order lifecycle states
- `OrderSide` - BUY/SELL enumeration

---

### [market_discovery.py](market_discovery.py)
**Purpose:** Market discovery via Gamma API
- Query Gamma API for events/markets
- Filter for BTC 15-minute markets
- Parse market metadata
- Extract timestamps
- Validate market duration

**Key Class:**
- `MarketDiscovery` - API client wrapper

**Key Methods:**
- `discover_btc_15m_markets()` - Main discovery
- `_is_btc_15m_market()` - Validation logic
- `_parse_market()` - Data extraction

---

### [order_manager.py](order_manager.py)
**Purpose:** Order operations via CLOB API
- CLOB client initialization
- Order placement
- Order monitoring
- Order cancellation
- Balance checking
- Allowance management

**Key Class:**
- `OrderManager` - CLOB wrapper

**Key Methods:**
- `place_liquidity_orders()` - Place 4 orders
- `check_order_status()` - Monitor fills
- `cancel_orders()` - Cancel unfilled
- `get_usdc_balance()` - Balance query

---

### [dashboard.py](dashboard.py)
**Purpose:** FastAPI web dashboard
- Real-time monitoring interface
- REST API endpoints
- Bot state queries
- Background bot thread
- Auto-refresh logic

**Key Endpoints:**
- `GET /` - Dashboard HTML
- `GET /api/status` - Bot status
- `GET /api/markets` - Active markets
- `GET /api/orders` - Order data
- `GET /api/logs` - Log entries

---

## 🌐 Web Interface

### [templates/dashboard.html](templates/dashboard.html)
**Purpose:** Web dashboard UI
- Responsive HTML5 layout
- Real-time data display
- Auto-refresh JavaScript
- Dark theme styling
- Market countdowns
- Order tables
- Live logs viewer

**Features:**
- Status bar with key metrics
- Market list with countdowns
- Order tracking tables
- Log streaming
- 5-second auto-refresh

---

## ⚙️ Configuration Files

### [.env.example](.env.example)
**Purpose:** Example configuration template
- Shows all available settings
- Documents default values
- Explains each parameter
- Template for user's `.env`

**Copy to `.env` and edit:**
```bash
cp .env.example .env
```

---

### [requirements.txt](requirements.txt)
**Purpose:** Python dependencies
- `py-clob-client` - Polymarket client
- `fastapi` - Web framework
- `uvicorn` - ASGI server
- `pydantic` - Data validation
- `requests` - HTTP client
- `web3` - Blockchain interactions
- Other dependencies

**Install:**
```bash
pip install -r requirements.txt
```

---

### [.gitignore](.gitignore)
**Purpose:** Git ignore rules
- `.env` - Never commit private keys
- `__pycache__/` - Python cache
- `*.pyc` - Compiled Python
- `venv/` - Virtual environment
- `bot.log` - Log files
- IDE files

---

## 📚 Documentation Files

### [README.md](README.md)
**Purpose:** Main project documentation
- Feature overview
- Installation guide
- Configuration reference
- Usage examples
- API references
- Safety notes

**Audience:** All users

---

### [QUICKSTART.md](QUICKSTART.md)
**Purpose:** 5-minute setup guide
- Step-by-step installation
- Quick configuration
- First run guide
- Troubleshooting basics
- Safety checklist

**Audience:** New users

---

### [GETTING_STARTED.md](GETTING_STARTED.md)
**Purpose:** Visual walkthrough
- Detailed setup with diagrams
- Expected behavior at each phase
- Dashboard screenshots
- Configuration examples
- Common scenarios

**Audience:** Visual learners

---

### [ARCHITECTURE.md](ARCHITECTURE.md)
**Purpose:** Technical deep-dive
- System architecture
- Component descriptions
- Data flow diagrams
- Threading model
- Error handling strategy
- Performance optimization
- Testing strategy

**Audience:** Developers

---

### [TROUBLESHOOTING.md](TROUBLESHOOTING.md)
**Purpose:** Problem solving guide
- Common issues and solutions
- Error message explanations
- Debugging steps
- Configuration problems
- Connection issues
- Order placement issues

**Audience:** Users encountering problems

---

### [PROJECT_SUMMARY.md](PROJECT_SUMMARY.md)
**Purpose:** Project overview
- Feature checklist
- Tech stack
- File structure
- Workflow diagram
- Statistics
- Status report

**Audience:** Project reviewers

---

### [FILE_REFERENCE.md](FILE_REFERENCE.md)
**Purpose:** This file
- Complete file listing
- Purpose of each file
- Usage instructions
- Relationships

**Audience:** Navigating the codebase

---

## 🔧 Utility Scripts

### [test_connection.py](test_connection.py)
**Purpose:** Test setup and connections
- Validate configuration
- Test Gamma API connection
- Test CLOB API connection
- Check wallet balance
- Comprehensive diagnostics

**Usage:**
```bash
python test_connection.py
```

**Tests:**
1. Configuration loading
2. Gamma API connectivity
3. CLOB client initialization
4. USDC balance retrieval

---

### [run.bat](run.bat)
**Purpose:** Windows launcher script
- Create virtual environment
- Install dependencies
- Start bot automatically

**Usage:**
```cmd
run.bat
```

**Platform:** Windows

---

### [run.sh](run.sh)
**Purpose:** Unix/Mac launcher script
- Create virtual environment
- Install dependencies
- Start bot automatically

**Usage:**
```bash
chmod +x run.sh
./run.sh
```

**Platform:** Linux, macOS

---

## 🛠 Development Files

### [setup.py](setup.py)
**Purpose:** Python package setup
- Package metadata
- Dependency specification
- Entry points
- Installation configuration

**Usage:**
```bash
pip install -e .  # Editable install
```

---

### [LICENSE](LICENSE)
**Purpose:** MIT License
- Open source license
- Usage terms
- Liability disclaimer

---

## 📊 File Relationships

### Execution Flow
```
main.py
  ├─→ config.py (load settings)
  ├─→ logger.py (setup logging)
  └─→ dashboard.py
       ├─→ bot.py (background thread)
       │    ├─→ market_discovery.py
       │    └─→ order_manager.py
       └─→ templates/dashboard.html
```

### Data Flow
```
Gamma API
  ↓
market_discovery.py
  ↓
models.py (Market objects)
  ↓
bot.py (decision making)
  ↓
order_manager.py
  ↓
CLOB API → Polygon Blockchain
```

### Configuration Flow
```
.env file
  ↓
config.py
  ↓
Used by: bot.py, order_manager.py, dashboard.py, logger.py
```

---

## 🎯 File Usage by Task

### First-Time Setup
1. Read: [QUICKSTART.md](QUICKSTART.md)
2. Copy: [.env.example](.env.example) → `.env`
3. Run: [test_connection.py](test_connection.py)
4. Run: [main.py](main.py)

### Daily Operation
- Run: [main.py](main.py)
- View: [templates/dashboard.html](templates/dashboard.html) (via browser)
- Monitor: `bot.log` (created automatically)

### Troubleshooting
1. Check: `bot.log`
2. Run: [test_connection.py](test_connection.py)
3. Read: [TROUBLESHOOTING.md](TROUBLESHOOTING.md)
4. Review: [config.py](config.py) settings

### Development
- Read: [ARCHITECTURE.md](ARCHITECTURE.md)
- Modify: [bot.py](bot.py), [order_manager.py](order_manager.py), [models.py](models.py)
- Test: [test_connection.py](test_connection.py)
- Install: [setup.py](setup.py)

### Understanding the System
1. Overview: [PROJECT_SUMMARY.md](PROJECT_SUMMARY.md)
2. Setup: [GETTING_STARTED.md](GETTING_STARTED.md)
3. Details: [README.md](README.md)
4. Technical: [ARCHITECTURE.md](ARCHITECTURE.md)
5. Reference: This file

---

## 📝 File Statistics

| Category | Count | Files |
|----------|-------|-------|
| Python Core | 7 | main.py, bot.py, config.py, logger.py, models.py, market_discovery.py, order_manager.py |
| Web Interface | 2 | dashboard.py, templates/dashboard.html |
| Documentation | 6 | README.md, QUICKSTART.md, GETTING_STARTED.md, ARCHITECTURE.md, TROUBLESHOOTING.md, PROJECT_SUMMARY.md |
| Configuration | 3 | .env.example, requirements.txt, .gitignore |
| Utilities | 3 | test_connection.py, run.bat, run.sh |
| Development | 2 | setup.py, LICENSE |
| **Total** | **23** | All files |

---

## 🔍 Finding What You Need

**"How do I get started?"**
→ [QUICKSTART.md](QUICKSTART.md)

**"I need visual guides"**
→ [GETTING_STARTED.md](GETTING_STARTED.md)

**"How does this work?"**
→ [ARCHITECTURE.md](ARCHITECTURE.md)

**"Something's broken!"**
→ [TROUBLESHOOTING.md](TROUBLESHOOTING.md)

**"Where's the configuration?"**
→ [config.py](config.py), [.env.example](.env.example)

**"How do orders work?"**
→ [order_manager.py](order_manager.py)

**"How are markets discovered?"**
→ [market_discovery.py](market_discovery.py)

**"What's the main logic?"**
→ [bot.py](bot.py)

**"How do I test it?"**
→ [test_connection.py](test_connection.py)

**"What are the data structures?"**
→ [models.py](models.py)

**"Where's the dashboard code?"**
→ [dashboard.py](dashboard.py), [templates/dashboard.html](templates/dashboard.html)

---

## 📦 Complete File List

1. [main.py](main.py) - Entry point
2. [bot.py](bot.py) - Bot logic
3. [config.py](config.py) - Configuration
4. [logger.py](logger.py) - Logging
5. [models.py](models.py) - Data models
6. [market_discovery.py](market_discovery.py) - Market discovery
7. [order_manager.py](order_manager.py) - Order management
8. [dashboard.py](dashboard.py) - Web dashboard
9. [templates/dashboard.html](templates/dashboard.html) - Dashboard UI
10. [.env.example](.env.example) - Config template
11. [requirements.txt](requirements.txt) - Dependencies
12. [.gitignore](.gitignore) - Git ignore
13. [README.md](README.md) - Main docs
14. [QUICKSTART.md](QUICKSTART.md) - Quick guide
15. [GETTING_STARTED.md](GETTING_STARTED.md) - Visual guide
16. [ARCHITECTURE.md](ARCHITECTURE.md) - Technical docs
17. [TROUBLESHOOTING.md](TROUBLESHOOTING.md) - Problem solving
18. [PROJECT_SUMMARY.md](PROJECT_SUMMARY.md) - Project overview
19. [FILE_REFERENCE.md](FILE_REFERENCE.md) - This file
20. [test_connection.py](test_connection.py) - Connection tester
21. [run.bat](run.bat) - Windows launcher
22. [run.sh](run.sh) - Unix launcher
23. [setup.py](setup.py) - Package setup
24. [LICENSE](LICENSE) - MIT license

---

**Everything you need is here. Happy coding!** 🚀
