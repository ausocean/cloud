# Config Requests
Config request should be made by a NetSender client when it is unconfigured, or when an update request has been made through a response code from another endpoint.

When the client receives a non-error from the NetReceiver server, the client should store the returned configuration in non-volatile storage, such that the configuration persists over a reboot or power-cycle. NetSender clients are responsible for managing the configuration, as subsequent config requests without the stored device key will be rejected, unless specifically requested by the server.

## Request
**GET** ```/config```

| Param | Type | Description | Example |
| ----- | ---- | ----------- | ------- |
| `ma` | `string` | Client MAC address. | `ma=A0:A0:A0:12:34:56` |
| `dk` | `int64` | Client Device key as prescribed by the service. Unconfigured devices will use a device key of 0. | `dk=1234567890` |
| `vn` | `string` | Client Version Number. This is up to the NetSender implemetation to choose the format. (_optional_) | `vn=123`, `vn=1.0.0` |
| `ut` | `int64` | Client uptime. This is reported in seconds since last reboot. (_optional_)<sup>1</sup> | `ut=120` |
| `la` | `string` | Client local IP address (_optional_) | `192.168.1.10` |
| `vt` | `int64` | **Deprecated**: _Length of variable types included in the request body in bytes (_optional_)_ <sup>2</sup> | `vt=128` |
| `md` | `string` | Client mode (_optional_) | `md=Normal` |
| `er` | `string` | Client error, if any (_optional_) | `er=LowVoltage` |

> <sup>1</sup> _Whilst reporting client uptime is not strictly enforced by a NetReceiver Server config endpoint, implementations often use the last update time of the uptime variable to infer the current status of the client. Uptime is also sent via poll requests routinely._

> <sup>2</sup> _VarTypes are still handled by some NetReceiver servers, however, support for dynamic variable types is being deprecated. Versioned static variable types are favoured, however implementation details have yet to be finalised._

### Device Key Authentication
The config request is designed to bootstrap a new, unconfigured device with a cloud defined configuration, as well as update the configuration of a previously configured device. A config request is authenticated using the device key and MAC address pair. However, for an unconfigured device, using the default device key of 0, with the unique device MAC is accepted. The server can also trigger an update using a response code, in this case, the first config request for the matching MAC address is also accepted with a default MAC, to allow a previously configured device to reconfigure if it had lost its saved config.

## Response
For a valid, successful configuration request, the response is a JSON representation of the device config. The response JSON also sends some additional fields to dynamically control the device.

| Key  | Type     | Description |
| --- | --- | --- |
| `ma` | `string` | Client MAC Address |
| `wi` | `string` | WiFi authentication, comma seperated SSID, and password |
| `ip` | `string` | Client Inputs, comma seperated pin names |
| `op` | `string` | Client Outputs, comma seperateed pin names  |
| `mp` | `int`    | Client Monitor Period, time in seconds between sensor measurements and subsequent poll requests |
| `ap` | `int`    | Client Act Period, time in seconds between actuator updates via act request |
| `ct` | `string` | Client Type as configured by NetReceiver service |
| `cv` | `string` | Client Version |
| `vs` | `int64`  | Server [varsum](./vars#varsum-calculation), computed checksum of the current variables registered for a device |
| `ts` | `int64`  | Server Unix Timestamp, can be used to synchronise client clock to server time |
| `dk` | `string` | Device Key, only returned for unconfigured devices |
| `rc` | `int`    | Response code, used to trigger device behaviour |

An example config response looks like the following:
```JSON
{
  "ma": "A0:A0:A0:12:34:56",
  "wi": "WiFiSSID,wifiPassword123",
  "ip": "A0,A1,T0,S0",
  "op": "D1,D2,D3",
  "mp": 60,
  "ap": 60,
  "ct": "Hydrophone",
  "cv": "1.2.3",
  "vs": -1329464821,
  "ts": 1769732878,
  "dk": "12345678"
}
```

## Errors
The `/config` endpoint returns errors in a JSON format, with two keys:
| Key  | type     | Description |
| ---  | ---      | ---         |
| `er` | `string` | Error string describing the error |
| `rc` | `int`    | Response Code (if any) |

### MAC Errors
A NetSender client cannot receive a configuration from a NetReceiver service until the device has been created and registered on the server. Some NetSender clients and NetReceiver servers have implementations to allow for dynamic device creation, however, this has not been formalised at the time of writing. 

If the server does not have a registered device for the requested MAC Address, this will return an error.
```JSON
{"er":"device not found"}
```

If the requested MAC address is not a valid MAC address, this will return an error.
```JSON
{"er":"invalid MAC address"}
```

### Device Key Errors
The device key is used by the NetReceiver Server to authenticate the client. If the incoming request is for an already configured device, that has not been marked as unconfigured, the request must contain the device key that matches the requested MAC.

If the Device Key does not match the Device Key of the registered device for the requested MAC address, or is not included at all this will return an error (with a response code to reconfigure).
```JSON
{"er":"invalid device key","rc":1}
```

If the Device Key is not an integer, and is malformed, this will return an error (with a response code to reconfigure).
```JSON
{"er":"malformed device key","rc":1}
```
