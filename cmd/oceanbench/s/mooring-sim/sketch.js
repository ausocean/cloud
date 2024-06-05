/*
AUTHORS
  Trek Hopton <trek@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean)

  This is free software: you can redistribute it and/or modify it
  under the terms of the GNU General Public License as published by
  the Free Software Foundation, either version 3 of the License, or
  (at your option) any later version.

  It is distributed in the hope that it will be useful,
  but WITHOUT ANY WARRANTY; without even the implied warranty of
  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
  GNU General Public License for more details.

  You should have received a copy of the GNU General Public License
  in gpl.txt. If not, see http://www.gnu.org/licenses/.
*/

// This sketch and its variables are based on AusOcean's Ops Mooring Calculations spreadsheet.
// ss is suggested separation, os is overwritten separation. 
let w, y, d, c, ss, os, a, b, h, l;
let overwritten = false;

// Scale.
let sc;

// Text inputs.
let depth;
let seas;
let separation;

// Wave params.
let amplitude
let frequency
let phaseShift
let period

function setup() {
  winw = window.innerWidth;
  winh = window.innerHeight;
  createCanvas(winw, winh);
  
  // Scale is 40x by default.
  sc = 40

  // Set initial inputs.
  w = 1.15;
  y = 1.17;
  d = 10;
  c = 3;
  os = 2*(d+c-y)+w;
  
  doCalcs();

  // Setup waves.
  amplitude = c*sc/2; // Amplitude of the sine wave.
  frequency = 0.005; // Frequency of the sine wave.
  phaseShift = 0; // Phase shift of the sine wave.
  period = 0.01;
  
  let indent = 400;
  depth = createInput(d.toString());
  depth.position(indent, 25);
  depth.size(200);
  depth.changed(updateDepth);
  
  seas = createInput(c.toString());
  seas.position(indent, 45);
  seas.size(200);
  seas.changed(updateSeas);
  
  separation = createInput(os.toString());
  separation.position(indent, 85);
  separation.size(200);
  separation.changed(updateSeparation);
  
  slider = createSlider(0, 100, sc);
  slider.position(winw-350, 45);
  slider.style('width', '200px');
  
  // Add an event listener to detect changes in the slider value.
  slider.input(updateSliderValue);
}

function draw() {
  background(180,230,255);

  // Inputs.
  strokeWeight(0);
  textSize(15);
  fill(0);
  text("max depth:", 40, 40);
  text("max seas:", 40, 60);
  text("suggested separation:                                                   " + ss.toFixed(2), 40, 80);
  text("override separation:", 40, 100);
  text("mooring line length (with 0.5m slack):                           " + l.toFixed(2), 40, 120);
  
  // Scale.
  text("scale (pixel:meter): " + sc, winw-350, 45);
  stroke(0);
  strokeWeight(1);
  line(winw-200-1*sc, 80, winw-200, 80);
  strokeWeight(0);
  text("1m", winw-180, 85);

  // Seafloor.
  let bedh = 200;
  fill(255,230,180);
  rect(0,winh-bedh, winw, bedh-1);

  // Vertical ruler
  let tickPos = winh - bedh;
  let tickLabel = 0;
  while (tickPos >= 0) {
    strokeWeight(1);
    stroke(0);
    fill(0);
    line(winw, tickPos, winw - 10, tickPos);
    textAlign(RIGHT, CENTER);
    strokeWeight(0);
    text(tickLabel + "m", winw - 25, tickPos);
    tickPos-=sc;
    tickLabel+=1;
  }
  textAlign(LEFT, BASELINE);

  // Sea.
  stroke(100);
  strokeWeight(1);
  amplitude = c*sc/2; // Amplitude of the sine wave.
  let maxSeaPos = winh-bedh-sc*(d+c);
  let minSeaPos = winh-bedh-sc*d;
  line(0, maxSeaPos, winw, maxSeaPos);
  line(0, minSeaPos, winw, minSeaPos);

  noFill();
  let waveRef = maxSeaPos + amplitude + amplitude * sin(frequency * winw/2 + phaseShift);
  beginShape();
  for (let xx = 0; xx < winw; xx++) {
    let yy = amplitude * sin(frequency * xx + phaseShift);
    vertex(xx, maxSeaPos + amplitude + yy);
  }
  endShape();

  // Rig.
  stroke(0);
  // Set rigRef to waveRef if you want the rig position to follow the wave.
  let rigRef = maxSeaPos;
  let pontoonw = 1.5;
  let pontoonh = 0.15;
  fill(255,255,255);
  rect(winw/2-(pontoonw/2)*sc,rigRef-pontoonh/2*sc,pontoonw*sc,pontoonh*sc);
  let mastw = 0.09;
  let masth = 1;
  fill(250,220,100);
  rect(winw/2-(mastw*sc)/2,rigRef-masth*sc-(sc*pontoonh)/2,mastw*sc,masth*sc);

  // Screw Piles.
  pileh = 0.75;
  line(winw/2-a*sc-(w*sc)/2,winh-bedh,winw/2-a*sc-(w*sc)/2,winh-bedh+pileh*sc);
  line(winw/2+a*sc+(w*sc)/2,winh-bedh,winw/2+a*sc+(w*sc)/2,winh-bedh+pileh*sc);

  // Bridle bar.
  fill(100);
  let bridleh = 0.025;
  rect(winw/2-(w/2)*sc,rigRef+y*sc-bridleh/2*sc,w*sc,bridleh*sc);

  // Mooring lines.
  line(winw/2-a*sc-(w*sc)/2,winh-bedh,winw/2-(w*sc)/2,rigRef+y*sc);
  line(winw/2+a*sc+(w*sc)/2,winh-bedh,winw/2+(w*sc)/2,rigRef+y*sc);

  // Bridles.
  line(winw/2-(pontoonw*sc)/2,rigRef,winw/2-(w*sc)/2,rigRef+y*sc);
  line(winw/2+(pontoonw*sc)/2,rigRef,winw/2+(w*sc)/2,rigRef+y*sc);

  phaseShift+=period;
}

function doCalcs(){
  ss = 2*(d+c-y)+w;
  let s;
  if(overwritten){
    s = os;
  } else {
    s = ss;
  }
  a = (s-w)/2;
  b = d+c-y;
  h = sqrt(pow(a, 2) + pow(b, 2));
  l = h+0.5;
  // Print spreadsheet values.
  console.log("s1: ", ss);
  console.log("a: ", a);
  console.log("b: ", b);
  console.log("h: ", h);
  console.log("l: ", l);
}

function updateDepth(){
  d = constrain(parseFloat(depth.value()), 0, 100);
  depth.value(d);

  // If the user hasn't overwritten the separation, update the separation text input.
  doCalcs();
  if(!overwritten){
    separation.value(ss.toString());
  }
}

function updateSeas(){
  c = constrain(parseFloat(seas.value()), 0, 20);
  seas.value(c);
  doCalcs();

  // If the user hasn't overwritten the separation, update the separation text input.
  if(!overwritten){
    separation.value(ss.toString());
  }
}

function updateSeparation(){
  os = constrain(parseFloat(separation.value()), w, 100);
  overwritten = true;
  separation.value(os.toString());
  doCalcs();
}

function updateSliderValue() {
  let sliderValue = constrain(slider.value(), 1, 100);
  slider.value(sliderValue);
  sc = sliderValue;
}