# OceanBench

OceanBench is the User Interface of the cloud services. It allows users to create, control, and monitor their IoT devices, as well as manage broadcasting. The deployed version of OceanBench is referred to as CloudBlue, to avoid confusion we will refer to CloudBlue throughout this guide.

## Navigation

The CloudBlue navigation window is accessible through the hamburger menu (top-left), where each of the different pages will be shown. The list of pages shown will be dependent on which pages the user has access to on the currently selected site. Using the site selection widget (top middle) the different sites the user has access to can be selected.

![Site Selection (public)](../images/bench-site-select.png)

## Terms and Naming

CloudBlue uses a few terms which have specific meanings in the context of the cloud services.

|Name|Definition|
|---|---|
|Site| A place where a CloudBlue device is deployed|
|Device| A microcontroller or other computer, with one or more pins associated with sensors or actuators|
|Sensor| A device associated with a pin which measures a physical property, _e.g. depth_|
|Actuator| An object associated with a pin which performs a physical action, _e.g., a relay_|
|Trigger| An action performed when a sensor condition occurs|
|Cron| An action performed at a certain time|
|Signal| A raw pin value, for a specific sensor and time|
|Variable| A user-defined variable for storing state|
|BinaryData| Arbitrary binary data, up to 1MB in size|

### Permissions

There are 3 permission levels for a user on a given site, Read, Write and Admin.

- **Read:** gives the user permission to search, view, and download data from the site
- **Write:** gives the user permission to upload data to the site
- **Admin:** gives the user permission to change settings for the site, the devices on the site, as well
  as add other users to the site.

## Pages

This section gives a brief overview of each of the pages on CloudBlue and their function.

### Search

The search page is used to search for data associated with a site. Typically this is data that was collected by a sensor assigned to the site. This page lets the user select the device, sensor (Pin), and time range for the data collected.

### Monitor

The monitor page gives an overview of the devices associated with a site. It provides the status of each device, as well as the most recently reported values for any sensor on the device. This page can be used to assess the current health and status of the site.

### Play

The Play page provides a platform designed to handle the playback, and filtering of audio/visual data. Whilst data can be uploaded to this page to be played back, it is more commonly used to playback data collected by devices on a site.

### Upload

The Upload page is used to upload 3rd party collected data to the datastore.

### Devices

The Devices page is used to change the settings of configuration of a device. This is where, variables, sensors, actuators, and modes are all assigned and set.

### Crons

The Crons page is where users can set recurring events. Events can be scheduled according to the [cron specification](https://www.ibm.com/docs/en/db2-as-a-service?topic=task-unix-cron-format). These events can perform different actions, which allow devices to be controlled, or checked at periodic intervals.

### Site

The Site page provides access to edit the site settings, as well as add new users to the site.

### Broadcast

The Broadcast page is where users can create and manage broadcasts.

### Configuration

The Configuration page is used to configure new devices. It is only accessible on the Sandbox site. See [configuration](/oceanbench/configuration) for more.

### Utilities

The Utilities page provides a handful of functions to help find and move devices, as well as calculating Media/Data IDs, and encoding MACs.
