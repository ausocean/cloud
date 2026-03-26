# MPEG-TS Media

MPEG-TS (MTS) is the container format used for media data eg. audio, video, in AusOcean's cloud services.

## Overview

The `MtsMedia` type represents a clip of continuous media data.

### Media ID (MID)

The Media ID is a unique identifier for the media source. It is constructed from:
1.  **MAC Address**: The 48-bit hardware address of the source device, encoded in a network-endian (big endian) 64-bit int.
2.  **Pin**: The media type identifier, which is a letter followed by a number. The letter can only be one of the following:
- V for Video
- S for Sound / Audio
- T for Text
- B for Binary (General binary information originally used for audio)

The number can be 0, 1, 2 or 3.
This is encoded into the MID following the MAC as a 4-bit nibble; two bits for the number, followed by two bits for the letter.

*Note: This is our standard way of forming a unique identifier but this could be expanded or changed in the future.*

### MTSFragment

An `MTSFragment` is a collection of continuous `MtsMedia` clips. While individual `MtsMedia` clips are limited to 1MB due to Datastore blob limits, an `MTSFragment` can span multiple clips to represent a larger logical segment of media.

## Storage and Segmentation

To accommodate Datastore's 1MB limit for blob properties, large media streams are automatically split into smaller chunks.

### Splitting Logic

Clips are split at:
1.  **PAT/PMT Boundaries**: To ensure each chunk is a valid MPEG-TS stream starting with necessary program data.
2.  **Discontinuities**: Whenever the media stream indicates a jump in time or a source change.
3.  **Size Limits**: Before exceeding the 1MB threshold.