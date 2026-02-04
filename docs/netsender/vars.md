# Vars Requests
Vars requests are used for a client to request updated variables values from the server. These requests are triggered by a varsum (`vs`) difference between a response from another request (typically a poll request), and the clients saved varsum. When the varsum changes, this indicates to the client that its local store of variables is stale, and that new variables are available on the server. 

Typically this results in the client making a request for new vars immediately, however, this is up to the implementation to decide what makes the most sense for their implementation.

There is no periodic querying of the vars endpoint, and this request will only be made in normal operation due to a change in varsum in order to conserve bandwidth more effectively.

## Request
**GET** ```/vars```

| Param | Type | Description | Example |
| ----- | ---- | ----------- | ------- |
| `ma` | `string` | Client MAC address. | `ma=A0:A0:A0:12:34:56` |
| `dk` | `int64` | Client Device key as prescribed by the service. | `dk=1234567890` |

### Authentication
A vars request must come from a configured device which has been authenticated with the NetReceiver server with a prescribed device key. The incoming request must contain a valid MAC address and device key pair, otherwise the request is rejected.

## Response
For a valid, successful vars request, the response is a JSON object with a list of variables and their values as well as additional server data.

| Key  | Type     | Description |
| --- | --- | --- |
| `id` | `string` | Client MAC Address (sans colons) |
| `vs` | `int64` | Server varsum for device variables |
| `rc` | `int`   | Response code |
| `ts` | `int64` | Server Timestamp (unix) |
| `<VarName>` | `string` | Server state of variable values.<sup>3</sup>

An example config response looks like the following:
```JSON
{
  "id": "a0a0a0123456",
  "rc": "1",
  "ts": "1770168412",
  "a0a0a0123456.AlarmPeriod": "600",
  "a0a0a0123456.AlarmVoltage": "855",
  "vs": "-22626666"
}
```

This response has returned all variables for the device, which in this case are `AlarmPeriod` and `AlarmVoltage`, with values 600, and 855 respectively. The variables are returned with their scoped name, which is prefixed with the device mac without colons. This is also returned in the ID field, to aid the device in parsing the variables.

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
