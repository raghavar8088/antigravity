import os
import requests
import zipfile
from pathlib import Path
from datetime import datetime, timedelta

# Binance Public Vision URL framework for Monthly Klines (Candles)
BASE_URL = "https://data.binance.vision/data/spot/monthly/klines"
SYMBOL = "BTCUSDT"
TIMEFRAME = "1m"
DATA_DIR = Path(__file__).parent.parent / "data"

def download_month(year: int, month: int):
    month_str = f"{month:02d}"
    filename = f"{SYMBOL}-{TIMEFRAME}-{year}-{month_str}.zip"
    url = f"{BASE_URL}/{SYMBOL}/{TIMEFRAME}/{filename}"
    
    zip_path = DATA_DIR / filename
    csv_path = DATA_DIR / filename.replace(".zip", ".csv")

    if csv_path.exists():
        print(f"[SKIP] {csv_path.name} already unzipped and stored locally.")
        return

    print(f"[DOWNLOAD] Fetching public archive: {url}")
    response = requests.get(url, stream=True)
    
    if response.status_code == 200:
        with open(zip_path, "wb") as f:
            for chunk in response.iter_content(chunk_size=1024*1024): # 1MB chunks
                f.write(chunk)
        
        print(f"[UNZIP] Extracting data from {filename}...")
        with zipfile.ZipFile(zip_path, 'r') as zip_ref:
            zip_ref.extractall(DATA_DIR)
            
        print(f"[CLEANUP] Removing raw payload {filename} from disk...")
        os.remove(zip_path)
    else:
        print(f"[ERROR] Binance missing data archive for {year}-{month_str}. HTTP {response.status_code}")

def main():
    DATA_DIR.mkdir(exist_ok=True)
    
    # Securely download the last 6 months of 1-minute BTC candles
    today = datetime.now()
    for i in range(1, 7):
        target_date = today - timedelta(days=30*i)
        download_month(target_date.year, target_date.month)

    print("\n[SUCCESS] Historical CSVs securely fetched into /data/ directory!")

if __name__ == "__main__":
    main()
