# NetSender Protocol

The NetSender Protocol is an IoT protocol designed to be simple to implement on a range of devices, completing a range of tasks. IoT devices are NetSender Clients, which report to a NetReceiver Server.

## Implementing NetSender

AusOcean has implemented NetSender clients for Raspberry Pi based cameras, hydrophones, and speakers, as well as for ESP (8266 and 32) based controller boards. Implementations for Raspberry Pi devices are written in [golang](https://go.dev/), and are usable on linux based systems, not just Pis. The ESP clients are currently implemented using Arduino targetting ESP32 and ESP8266 devices. Both implementations can be found in the AusOcean client repository (available [here](https://github.com/ausocean/client)) and are licensed under the GNU General Public License.

New NetSender Clients can be implemented using AusOcean's implementations for Go and Arduino, or can be implemented from scratch using the defined protocol in this documentation. 

> [!NOTE] 
> Whilst AusOcean's implementations are licensed using GPL, future implementations are not required to follow this license. The license does NOT apply if the new implementation is developed independently by following the protocol specification. However, if the existing source code is used in an inspirational or reference capacity to directly derive, port, or translate the logic, the resulting work is considered a derivative work and must comply with the GPL. To maintain full licensing independence, we recommend developers work exclusively from the written protocol documentation. That said, as an organization committed to open science and environmental transparency, we strongly encourage developers to adopt the GPL license for their new implementations. Doing so ensures that the entire Netsender ecosystem remains open, collaborative, and beneficial to the global research community.

## Communicating with a NetReceiver

AusOcean runs services which implement the NetReceiver server architecture. [CloudBlue](https://bench.cloudblue.org) implements a user interface to configure and control NetSender clients reporting to DataBlue. To learn more contact [info@ausocean.org](mailto:info@ausocean.org).
