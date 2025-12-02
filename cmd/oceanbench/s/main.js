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
  var forms = document
    .getElementById(srcContainerID)
    .getElementsByTagName("form");
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

  const sign = offset[0] === '-' ? -1 : 1;
  const parts = offset.slice(1).split(':');
  const hours = parseInt(parts[0], 10);
  const minutes = parseInt(parts[1], 10);

  return sign * (hours + minutes / 60);
}

// sync syncs time and timestamp input values. If either of these inputs are changed, the other input is updated to match.
// pickerUsed signifies if the time picker was used to change the time.
function sync(timeID, tsID, tzID, pickerUsed) {
  var tz = document.getElementById(tzID).value;
  // set the timestamp to zero if it is empty (default UTC)
  if (tz == "") {
    document.getElementById(tzID).value = "0";
    tz = "0";
  }
  if (pickerUsed) {
    // Update timestamp from time picker
    var s = document.getElementById(timeID).value;
    if (s == "") {
      document.getElementById(tsID).value = "";
      return;
    }
    if (s.length == 5) {
      s += ":00"; // Append seconds to make RFC3339 compliant.
    }
    // If the timezone is not in UTC offset format, try to convert it from an offset number.
    if(!checkUTCOffset(tz)){
      s += tzFormatUTCOffset(tz);
    } else {
      s += tz;
    }
    // prepend a date to make it a valid datetime
    s = "2000-01-01T" + s
    // parse the datetime and convert it to seconds
    const ts = new Date(s).getTime() / 1000;
    document.getElementById(tsID).value = ts.toString();
  } else {
    // Update time picker from timestamp
    const timestamp = document.getElementById(tsID).value;
    // if we don't have a timestamp, we don't have a time
    if (timestamp == "") {
      document.getElementById(timeID).value = "";
      return;
    }
    // if the timezone is a valid UTC offset, convert it to hours
    if(checkUTCOffset(tz)){
      tz = tzParseUTCOffset(tz);
    }

    // add the offset (in seconds) to the timestamp
    const ts = parseInt(timestamp) + Math.round(parseFloat(tz) * 3600);
    // convert the timestamp to an iso string and extract the time portion
    const dt = new Date(ts * 1000).toISOString().slice(11,16);
    document.getElementById(timeID).value = dt;
  }
}

// checkUTCOffset checks if the timezone is in the format of a UTC offset.
function checkUTCOffset(tz) {
  const utcOffsetRegex = /^[+-](?:2[0-3]|[01][0-9]):[0-5][0-9]$/;
  return utcOffsetRegex.test(tz);
}
