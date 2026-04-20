import os

filepath = r"c:\Trading apllication\client\src\components\Nifty50OptionScalper.tsx"

# Read as latin-1 to handle the 0xB7 character correctly
with open(filepath, "r", encoding="latin-1") as f:
    content = f.read()

# Write back as UTF-8
with open(filepath, "w", encoding="utf-8") as f:
    f.write(content)

print(f"Fixed encoding for {filepath}")
