export class Device {
  Name: string = "";
  MAC: string = "";
  Type: string = "";
}

export class Devices {
  Controllers: Device[] = [];
  Cameras: Device[] = [];
}
