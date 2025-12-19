"""Shared RPC configuration loaded from environment variables."""

import os
from dotenv import load_dotenv

load_dotenv()

RPC_URL = os.getenv("RPC_URL", "https://polygon-rpc.com")
