# Response Codes

NetReceiver endpoints can return a response code in order to trigger the NetSender client to perform some action or task.

## Codes

| Code | Name     | Description                         |
| ---- | -------- | ----------------------------------- |
| 0    | OK       | Indicates success and configured    |
| 1    | Update   | Triggers `/config` request          |
| 2    | Reboot   | Triggers device to restart          |
| 3    | Debug    | Triggers device to debug            |
| 4    | Upgrade  | Triggers device to self-upgrade     |
| 5    | Alarm    | Triggers device to alarm            |
| 6    | Test     | Triggers device to do test function |
| 7    | Shutdown | Triggers device to shutdown         |

## Support

It is not required for every NetSender implementation to support every response code. Not every response code will make sense in a NetSender device. However the Update response code **MUST** be handled by every implementation of NetSender.
