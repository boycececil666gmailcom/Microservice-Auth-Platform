```text
+---------------------------------------------------------------------------+
| json.Marshal vs json.NewEncoder(w).Encode Data Flow                       |
|                                                                           |
| [Pattern 1: json.Marshal(value)] (Returns data into variable)             |
|   jsonData, err := json.Marshal(value)                                    |
|       |                                                                   |
|       v (Data is returned to variable in memory)                          |
|     jsonData --> Must explicitly call w.Write(jsonData)                   |
|                                                                           |
| [Pattern 2: json.NewEncoder(w).Encode(value)] (Streams directly to w)     |
|   _ = json.NewEncoder(w).Encode(value)                                    |
|   |                   |         |                                         |
|   |                   |         +-- Data source to encode                 |
|   |                   +-- Destination: Data is WRITTEN DIRECTLY to w (socket)|
|   v                                                                       |
|   Discarded value is ONLY the "error" return, NOT the JSON data!          |
+---------------------------------------------------------------------------+
```
