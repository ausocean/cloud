# Poll Requests
Poll requests should be made by a NetSender client once every monitor period, as defined by the client configuration. A Poll request is used to send Pin data from the client to the server, and receive any relevant server updates.

## Request
**GET** or **POST** ```/poll```

| Param | Type | Description | Example |
| ----- | ---- | ----------- | ------- |
| `ma` | `string` | Client MAC address. | `ma=A0:A0:A0:12:34:56` |
| `dk` | `int64` | Client Device key as prescribed by the service. | `dk=1234567890` |
| `ut` | `int64` | Client uptime. This is reported in seconds since last reboot. (_optional_)<sup>1</sup> | `ut=120` |
| `<PinName>` | `int64` or `float64` | Client pin value. For `Scalar` pins this is a measurement, for `Vector` pins this represents the length of the binary data (if any) in the request body <sup>2</sup>. | `A0=678` `T1=128`

> <sup>1</sup> _Whilst reporting client uptime is not strictly enforced by a NetReceiver Server poll endpoint, implementations often use the last update time of the uptime variable to infer the current status of the client. Uptime is also sent via poll requests routinely._

> <sup>2</sup> _To learn more about Pins and the difference between vector and scalar pins, see [Pins](../oceanbench/device-configuration#pins)_

### Authentication
A poll request must come from a configured device which has been authenticated with the NetReceiver server with a prescribed device key. The incoming request must contain a valid MAC address and device key pair, otherwise the request is rejected.

### Vector Pin Data
Unlike scalar pins which are pins which can be represented as non-negative numbers in the HTTP query parameters, vector pin data is sent in the body of the HTTP request. For a device sending vector data, requests to the `/poll` endpoint must be of type `POST`. Vector pins still have a query parameter component, however, the value of a vector pin query parameter is the length of the data sent in the body.

If there are multiple Vector pins configured on the same device, a poll request may contain more than one blob of data in the body. This is the reason for passing the binary length in the request, allowing the NetReceiver server to parse the correct blob per pin. When a NetReceiver server parses a request with multiple vector pins, it handles each pin value in the order which it is registered in the device configuration.

For example, a device with the Inputs, `A0,T1,B0`, may report two types of vector data in each poll request. Since `T1` is listed in the input list first, the vector data for the `T1` pin should appear first in the body, followed by the `B0` vector data.

## Response
For a valid, successful poll request, the response is a JSON value, containing the current actuator state, as well as the current [varsum](./vars#varsum-calculation) and device MAC for confirmation.

| Key  | Type     | Description |
| --- | --- | --- |
| `ma` | `string` | Client MAC Address |
| `vs` | `int64` | Server varsum for device variables |
| `rc` | `int`   | Response code to trigger device action |
| `<PinName>` | `Bool` | Server state of current actuator states.<sup>3</sup>

> <sup>3</sup> _Whilst current actuators are only boolean values, future plans for protocol expansion include pseudo actuators which can handle more complex actuation tasks._

An example config response looks like the following:
```JSON
{ 
  "D25": 0, 
  "D32": 1, 
  "D33": 0, 
  "ma": "A0:A0:A0:12:34:56", 
  "vs": -494042761,
}
```

This response indicates that `D25` and `D33` should be in their _low_<sup>4</sup> state, whilst `D32` should be in its _high_ state.

> <sup>4</sup> _Typically the _low_ state for a NetSender pin is the off state. However, depending on the electronics setup and logic of the control, the _low_ state may be the on state._

## Errors
The `/poll` endpoint returns errors in a JSON format, with two keys:
| Key  | type     | Description |
| ---  | ---      | ---         |
| `er` | `string` | Error string describing the error |
| `rc` | `int`    | Response Code (if any) |

### MAC and Device Key Errors
The MAC address and device key are used by the NetReceiver server to authenticate the incoming request, if the MAC address and Device key are invalid (both individually and as a pair), an error will be returned.

If the server does not have a registered device for the requested MAC Address, this will return an error.
```JSON
{"er":"datastore: no such entity"}
```

If the requested MAC address is not a valid MAC address, this will return an error.
```JSON
{"er":"invalid MAC address"}
```

If the Device Key does not match the Device Key of the registered device for the requested MAC address, this will return an error (with a response code to reconfigure).
```JSON
{"er":"invalid device key","rc":1}
```

If the Device Key is missing altogether, this will also return an error (with a response code to reconfigure).
```JSON
{"er":"missing device key","rc":1}
```

If the Device Key is not an integer, and is malformed, this will return an error (with a response code to reconfigure).
```JSON
{"er":"malformed device key","rc":1}
```

## Pin Errors
If the sent pin value cannot be parsed as a float64, this will return an error.
```JSON
{"er":"invalid value"}
```

If a vector pin length in the header is less then the length of the data in the body, this will return an error.

There are also a few other errors that can be returned from failing to read the vector data from the body.
```JSON
{"er":"EOF"}
```

If the device is configured with an input pin which does not have a defined standard, such as an `A`, `D`, `X`, `B`, `S`, `V`, `T` pin, this will return an error.
```JSON
{"er":"invalid pin"}
```
