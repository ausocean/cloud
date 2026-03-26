# Media ID (MID)

The Media ID (MID) is a unique identifier for a media source. It is constructed from:
1.  **MAC Address**: The 48-bit hardware address of the source device.
2.  **Pin**: The media type identifier, which is a letter followed by a number.

The letter can only be one of the following:

| Letter | Pin Type | Binary | Notes |
| :--- | :--- | :--- | :--- |
| `V` | Video | `00` | V0 for first video channel on a device |
| `S` | Sound / Audio | `01` | S0 for first audio channel on a device |
| `T` | Text | `10` | T0 for logs, T1 for GPS |
| `B` | Binary | `11` | Not currently used |

The number following the letter can be 0, 1, 2, or 3:

| Number | Binary |
| :--- | :--- |
| `0` | `00` |
| `1` | `01` |
| `2` | `10` |
| `3` | `11` |

The letter and number are encoded together into the MID following the MAC as a single 4-bit nibble, which is done by first shifting the letter's bits left by 2, then combining their binary values with a bitwise OR.

Here is a table of the full MID and its bits:
Bits | Name
--- | ---
0-47 | Device MAC
48-49 | Pin Letter
50-51 | Pin Number

## Generating a MID: A Step-by-Step Example

A Media ID (MID) is a single `int64` value that packs together a device's MAC address and a pin. 

To create a MID, the system parses the MAC address into a 48-bit integer, shifts it left by 4 bits to make room, and then drops a 4-bit pin identifier into that empty space.

Here is a step-by-step example using the MAC address `"00:00:00:00:00:0A"` and the pin `"B2"`.

### Step 1: Encode the MAC Address
First, `MacEncode` strips the colons from the MAC address and converts the resulting hex string into a network-endian `int64`.

* **Input:** `"00:00:00:00:00:0A"`
* **Hex representation:** `0x00000000000A`
* **Integer value:** `10`

### Step 2: Encode the Pin
Next, `putMtsPin` converts the 2-character string into a 4-bit nibble. The first character (the type) occupies the top two bits, and the second character (the number) occupies the bottom two bits.

* **Input:** `"B2"`
* **Determine Type:** The letter `'B'` maps to the top two bits as `11` (which is `0x0C` or decimal `12`).
* **Determine Number:** The digit `'2'` maps to the bottom two bits as `10` (which is `0x02` or decimal `2`).
* **Combine:** `0x0C | 0x02` results in **`0x0E`** (decimal `14`).

### Step 3: Combine into the Final MID
Finally, `ToMID` combines these two integers. It takes the encoded MAC, shifts it left by 4 bits (`<< 4`) to create a 4-bit gap at the end, and then uses a bitwise OR (`|`) to insert the encoded pin into that gap.

1.  **Shift the MAC:** Take the encoded MAC (`0x0A`) and shift it left by 4. 
    * `0x0A << 4` becomes **`0xA0`** (decimal `160`).
2.  **Insert the Pin:** Apply the bitwise OR with the encoded pin (`0x0E`).
    * `0xA0 | 0x0E` becomes **`0xAE`**
3.  **Final Integer:** The resulting hex value `0xAE` is returned as a standard decimal integer.
    * **Final MID:** `174`

Because the bit shift is exactly 4 bits (one hex character), you can easily see how they merged just by looking at the hex values: MAC `0xA` + Pin `0xE` = MID `0xAE`.
