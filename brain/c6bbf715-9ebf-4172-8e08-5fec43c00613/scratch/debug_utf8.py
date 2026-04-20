import os

filepath = r"c:\Trading apllication\client\src\components\OptionsScalper.tsx"
with open(filepath, "rb") as f:
    data = f.read()

try:
    data.decode("utf-8")
    print("Whole file is valid UTF-8")
except UnicodeDecodeError as e:
    print(f"File error: {e}")
    print(f"Error at index {e.start}: {data[e.start:e.start+16].hex(' ')}")
