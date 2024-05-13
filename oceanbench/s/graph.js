/*
DESCRIPTION
  graph.js provides javascript functionality for making data requests to
  netreceiver and then graphing the acquired data.

  This file requires the following dependencies, to be included in the template:
  https://cdn.amcharts.com/lib/4/core.js
  https://cdn.amcharts.com/lib/4/charts.js
  https://cdn.amcharts.com/lib/4/themes/animated.js
  https://cdn.rawgit.com/kimmobrunfeldt/progressbar.js/1.1.0/dist/progressbar.js

AUTHOR
  Saxon Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2021 the Australian Ocean Lab (AusOcean)

  It is free software: you can redistribute it and/or modify them
  under the terms of the GNU General Public License as published by the
  Free Software Foundation, either version 3 of the License, or (at your
  option) any later version.

  It is distributed in the hope that it will be useful, but WITHOUT
  ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or
  FITNESS FOR A PARTICULAR PURPOSE. See the GNU General Public License
  for more details.

  You should have received a copy of the GNU General Public License in gpl.txt.
  If not, see http://www.gnu.org/licenses.
*/

var data = [];
var queries = [];
var qCnt = 0;
var bar;

// graphHandler handles the collection and graphing of data for the specified
// mac, pin, site key and start and end times. This works recursively by handling
// itself to a callback function that the HTTP client uses once a response is
// received; a side effect of using XMLHttpRequest properly i.e. asynchronously.
function graphHandler(host, skey, mac, pin, s, f, tz, res) {
  // If first call, we'll set up a progress bar and prep the query strings.
  if (data.length == 0) {
    document.getElementById("graph-error").innerHTML = "";
    console.log("first graphHandler call");
    bar = new ProgressBar.Line("#progress", {
      easing: "easeInOut",
    });
    prepQueries(host, skey, mac, pin, s, f, tz, res);
  }

  // This means we've got all the data; graph and clear values for next time.
  if (qCnt == queries.length) {
    console.log("got all data");
    graph();
    data = [];
    queries = [];
    qCnt = 0;
    err = false;
    return;
  }

  // We have some data we need to get, so set up a HTTP request and have the
  // HTTP client callback this handler function.
  const tzUnix = parseFloat(tz) * 3600;
  asyncHTTPGet(
    queries[qCnt],
    function (response) {
      qCnt++;
      // Animate progress bar to indicate progress.
      bar.animate(qCnt / queries.length);

      // Need to parse the CSV response string and get components.
      var lines = response.split("\n");
      for (var j = 0; j < lines.length; j++) {
        var sub = lines[j].split(",");
        var date = timeFormatToDate(sub[0], tzUnix);
        var value = sub[1];
        data.push({
          date: date,
          value: value,
        });
      }
      // Have client callback this handler function.
      graphHandler(host, skey, mac, pin, s, f, tz, res);
    },
    function (xhr) {
      document.getElementById("graph-error").innerHTML =
        "HTTP error, status: " +
        xhr.statusText +
        " for URL: " +
        xhr.responseURL;
    },
  );
}

// prepQueries prepares a string array of the required queries to be made to get
// the data for the specified period. This is a side effect of netreceiver only
// being able to provide data of a period no more than 60 hours long.
function prepQueries(host, skey, mac, pin, s, f, tz, res) {
  console.log("preparing queries");
  const stUnix = parseInt(s);
  const ftUnix = parseInt(f);

  // Query URL characteristics.
  const request = "/data/";
  const exportFormat = "csv";

  // Timing characteristics.
  const maxSeconds = 60 * 3600.0;

  // Need to calculate how many 60 hour periods we'll need to get.
  var diff = ftUnix - stUnix;
  var nMaxPeriods = Math.ceil(diff / maxSeconds);
  var baseURL = host + request + skey;

  // Prep each query URL.
  for (var i = 0; i < nMaxPeriods; i++) {
    const s = stUnix + i * maxSeconds;
    const f = stUnix + (i + 1) * maxSeconds;

    var params = {
      ma: mac,
      pn: pin,
      do: exportFormat,
      ds: s.toString(),
      df: f.toString(),
      dr: res,
      tz: tz,
    };
    console.log(params);
    var queryParams = encodeQuery(params);
    var url = baseURL + "?" + queryParams;
    console.log("query prepared: %s", url);
    queries.push(url);
  }
}

// graph uses the am4charts module to create a line graph of the data stored
// in the global data array.
function graph() {
  console.log("graphing");
  document.getElementById("graph").innerHTML = `
    <div id="chartdiv"></div>
  `;

  am4core.ready(function () {
    // Themes begin
    am4core.useTheme(am4themes_animated);
    // Themes end
    var chart = am4core.create("chartdiv", am4charts.XYChart);

    chart.data = data;

    // Create axes
    var dateAxis = chart.xAxes.push(new am4charts.DateAxis());
    dateAxis.renderer.minGridDistance = 60;

    var valueAxis = chart.yAxes.push(new am4charts.ValueAxis());

    // Create series
    var series = chart.series.push(new am4charts.LineSeries());
    series.dataFields.valueY = "value";
    series.dataFields.dateX = "date";
    series.tooltipText = "{value}";

    series.tooltip.pointerOrientation = "vertical";

    chart.cursor = new am4charts.XYCursor();
    chart.cursor.snapToSeries = series;
    chart.cursor.xAxis = dateAxis;

    chart.scrollbarX = new am4core.Scrollbar();
  });
}

// asyncHTTPGet performs a HTTP GET request to the specified URL. This works
// asynchronously, and will use the callback to provide the response.
function asyncHTTPGet(theUrl, callback, errCallback) {
  console.log("HTTP GET");
  var xmlHttp = new XMLHttpRequest();
  xmlHttp.onreadystatechange = function () {
    if (xmlHttp.readyState == XMLHttpRequest.DONE) {
      if (xmlHttp.status == 200) {
        console.log("got response");
        callback(xmlHttp.responseText);
      } else {
        errCallback(xmlHttp);
      }
    }
  };
  xmlHttp.open("GET", theUrl, true); // true for asynchronous
  xmlHttp.send(null);
}

// encodeQuery simply encodes a series of query parameters into a single string.
function encodeQuery(data) {
  const ret = [];
  for (let d in data)
    ret.push(encodeURIComponent(d) + "=" + encodeURIComponent(data[d]));
  return ret.join("&");
}

// unixToTimeFormat takes a unix time and produces a time string in the form of
// "yyyy-mm-dd hh:mm".
function unixToTimeFormat(tUnix, tzUnix) {
  var date = new Date((tUnix - tzUnix) * 1000);
  var year = date.getFullYear();
  var month = date.getMonth() + 1;
  var day = date.getDate();
  var hours = date.getHours();
  var mins = date.getMinutes();

  if (month < 10) {
    month = "0" + month;
  }

  var dateTime = year + "-" + month + "-" + day + " " + hours + ":" + mins;
  return dateTime;
}

// timeFormatToDate produces a javascript Date object from a time of format
// "yyyy-mm-dd hh:mm".
function timeFormatToDate(time, tzUnix) {
  var parts = time.replace(/[-/s :]+/g, ",").split(",");
  var year = parts[0];
  var month = parts[1] - 1;
  var date = parts[2];
  var hours = parts[3];
  var minutes = parts[4];
  var dt = new Date(year, month, date, hours, minutes, 0, 0);
  dt.setTime(dt.getTime() + tzUnix);
  return dt;
}
