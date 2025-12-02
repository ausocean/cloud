// encodeMAC encodes a MAC address into a 48-bit integer.
function encodeMAC(mac) {
  var enc = BigInt(0);
  if (mac.length != 17) {
    console.log("invalid MAC: " + mac);
    return enc;
  }
  for (i = 15, shift = 0; i >= 0; i -= 3, shift += 8) {
    var n = BigInt(parseInt(mac.substring(i, i + 2), 16));
    enc += n << BigInt(shift);
  }
  return enc;
}

// encodeMediaPin encodes a pin into a 4-bit integer.
// Valid pins are V, S, T, or B followed by a single digit.
// See mtsmedia.go for an explanation of the encoding scheme.
function encodeMediaPin(pin) {
  var enc = Number(0);
  if (pin.length != 2) {
    console.log("invalid pin: " + pin);
    return enc;
  }
  switch (pin.charAt(0)) {
    case "V":
      break;
    case "S":
      enc = 4;
      break;
    case "T":
      enc = 8;
      break;
    case "B":
      enc = 12;
      break;
    default:
      console.log("invalid pin type: " + pin.charAt(0));
  }
  enc |= Number(pin.charAt(1) - "0");
  return enc;
}

// toMID returns a Media ID as a BigInt given a MAC address and a pin.
function toMID(mac, pin) {
  if (mac == "" || pin == "") {
    return 0;
  }
  return (encodeMAC(mac) << BigInt(4)) | BigInt(encodeMediaPin(pin));
}

// encodeScalarPin encodes a pin into a 4-bit integer.
// Valid pins are A, D or X followed by one or two digits.
// See scalar.go for an explanation of the encoding scheme.
function encodeScalarPin(pin) {
  if (pin.length != 2 && pin.length != 3) {
    console.log("invalid pin: " + pin);
    return Number(0);
  }
  var enc = Number(pin.substring(1));
  switch (pin.charAt(0)) {
    case "A":
      enc += 100;
      break;
    case "D":
      enc += 1;
      break;
    case "X":
      enc |= 0x80;
      break;
    default:
      console.log("invalid pin type: " + pin.charAt(0));
  }
  return enc;
}

// toSID returns a Scalar ID as a BigInt given a MAC address and a pin.
function toSID(mac, pin) {
  if (mac == "" || pin == "") {
    return 0;
  }
  return (encodeMAC(mac) << BigInt(8)) | BigInt(encodeScalarPin(pin));
}

// updateMID computes a Media/Scalar ID from a MAC and pin and updates the id element.
function updateMID(mac, pin, id) {
  var macElem = document.getElementById(mac);
  var pinElem = document.getElementById(pin);
  var idElem = document.getElementById(id);
  if (!macElem || !pinElem || !idElem) {
    return;
  }
  switch (pinElem.value.charAt(0)) {
    case "A":
    case "D":
    case "X":
      idElem.value = toSID(macElem.value, pinElem.value).toString();
      break;
    case "V":
    case "S":
    case "T":
    case "B":
      idElem.value = toMID(macElem.value, pinElem.value).toString();
      break;
    default:
      idElem.value = "";
      break;
  }
}

// copyFormValues copies field values from the selected form within srcContainerID to the form dstID.
function copyFormValues(dstID, srcContainerID, fields) {
  var dst = document.getElementById(dstID);
  var forms = document.getElementById(srcContainerID).getElementsByTagName("form");
  for (var ii = 0; ii < forms.length; ii++) {
    var src = forms[ii];
    var checkbox = src.elements["select"];
    if (checkbox && checkbox.checked) {
      for (var fld in fields) {
        if (fields[fld] == "input" || fields[fld] == "select") {
          dst.elements[fld].value = src.elements[fld].value;
        } else if (fields[fld] == "checkbox") {
          dst.elements[fld].value = src.elements[fld].checked;
        }
      }
      return true;
    }
  }
  alert("Nothing selected");
  return false;
}

// getTimezone returns the current timezone as string formatted +hh:mm.
function getTimezone() {
  var dt = new Date();
  var tz = dt.getTimezoneOffset();
  var ss;
  if (tz < 0) {
    ss = "+";
  } else {
    ss = "-";
  }
  tz = Math.abs(tz);
  var hh = Math.floor(tz / 60);
  var mm = tz % 60;
  if (hh < 10) {
    ss += "0" + hh.toString();
  } else {
    ss += hh.toString();
  }
  if (mm < 10) {
    ss += ":0" + mm.toString();
  } else {
    ss += ":" + mm.toString();
  }
  return ss;
}

// tzFormatUTCOffset formats a timezone offset number as a +/-hh:mm string.
function tzFormatUTCOffset(tz) {
  if (tz == "0") {
    return "Z";
  }
  const z = parseFloat(tz);
  const h = Math.floor(Math.abs(z));
  const m = (Math.abs(z) - h) * 60;
  const hh = h.toString().padStart(2, "0");
  const mm = m.toString().padStart(2, "0");
  if (tz < 0) {
    return "-" + hh + mm;
  } else {
    return "+" + hh + mm;
  }
}

// tzParseUTCOffset parses a +/-hh:mm string into a timezone offset number.
function tzParseUTCOffset(offset) {
  if (!offset) {
    return "";
  }

  if (offset === "Z" || offset === "+00:00" || offset === "-00:00") {
    return 0;
  }

  const sign = offset[0] === "-" ? -1 : 1;
  const parts = offset.slice(1).split(":");
  const hours = parseInt(parts[0], 10);
  const minutes = parseInt(parts[1], 10);

  return sign * (hours + minutes / 60);
}

// syncDateTime syncs a date/time picker and a timestamp input.
// If either of these are changed, the other is updated to match.
// pickerUsed signifies if the date/time picker was changed.
function syncDateTime(datetimeID, timestampID, timezoneID, pickerUsed) {
  let timezone = document.getElementById(timezoneID).value;
  // set the timestamp to zero if it is empty (default UTC)
  if (timezone == "") {
    document.getElementById(timezoneID).value = "0";
    timezone = "0";
  }
  const pickerElem = document.getElementById(datetimeID);
  const timestampElem = document.getElementById(timestampID);
  if (pickerUsed) {
    // Update timestamp from time picker
    let datetime = pickerElem.value;
    if (datetime == "") {
      timestampElem.value = "";
      return;
    }
    // time only
    if (datetime.length == 5) {
      datetime += ":00"; // Append seconds to make RFC3339 compliant.
      // prepend a date to make it a valid datetime
      datetime = "2000-01-01T" + datetime;
    }
    // date and time
    else if (datetime.length == 16) {
      datetime += ":00"; // Append seconds to make RFC3339 compliant.
    }
    // If the timezone is not in UTC offset format, try to convert it from an offset number.
    if (!checkUTCOffset(timezone)) {
      datetime += tzFormatUTCOffset(timezone);
    } else {
      datetime += timezone;
    }
    // parse the datetime and convert it to seconds
    const timestamp = new Date(datetime).getTime() / 1000;
    timestampElem.value = timestamp.toString();
  } else {
    // Update time picker from timestamp
    const timestamp = timestampElem.value;
    // if we don't have a timestamp, we don't have a time
    if (timestamp == "") {
      pickerElem.value = "";
      return;
    }
    // if the timezone is a valid UTC offset, convert it to hours
    if (checkUTCOffset(timezone)) {
      timezone = tzParseUTCOffset(timezone);
    }

    // add the offset (in seconds) to the timestamp
    const localTimestamp = parseInt(timestamp) + Math.round(parseFloat(timezone) * 3600);
    // convert the timestamp to an iso string and extract the time portion
    const datetime = new Date(localTimestamp * 1000).toISOString();
    if (pickerElem.type == "time") {
      // extract the time portion only (HH:mm)
      pickerElem.value = datetime.slice(11, 16);
    } else if (pickerElem.type == "datetime-local") {
      // extract the date and hours/mins only (YYYY-MM-DDTHH:mm)
      pickerElem.value = datetime.slice(0, 16);
    }
  }
}

// checkUTCOffset checks if the timezone is in the format of a UTC offset.
function checkUTCOffset(tz) {
  const utcOffsetRegex = /^[+-](?:2[0-3]|[01][0-9]):[0-5][0-9]$/;
  return utcOffsetRegex.test(tz);
}
