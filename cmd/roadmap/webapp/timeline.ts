import p5 from "p5";

import { fetchTasks, submitTasks } from "./data";
import { getOwnerColor, getPriorityColor } from "./config";

let tasks: any[] = [];
let visibleTasks: any[] = [];

let timelineStart: number;
let timelineEnd: number;
let startX = 100; // Left margin
let barHeight = 22;
let yBoxSpacing = 24;
let offsetX = 0; // Used for panning
let isDragging = false;
let dragStartX = 0;
let nowX = 0;
let redraw = false;
let zoomLevel = 2.5; // Default zoom level
let timelineTop = 0; // Updated in draw
let toolTipTask: any = null;

// p5.js sketch
const sketch = (p: p5) => {
  p.setup = async () => {
    try {
      tasks = await fetchTasks();
    } catch (error) {
      console.error("❌ Error fetching Gantt data:", error);
    }
    if (tasks.length === 0) {
      console.warn("⚠️ No tasks to display.");
      return;
    }
    // Initialise timeline range from the data; dateToX and the zoom init
    // below depend on these being set before any drawing/coordinate math.
    timelineStart = Math.min(...tasks.map((t) => new Date(t.start).getTime()));
    timelineEnd = Math.max(...tasks.map((t) => new Date(t.end).getTime()));

    // Check the 'hide past tasks' checkbox
    visibleTasks = [...tasks];
    const hidePastTasks = document.getElementById("hide-past-tasks") as HTMLInputElement;
    function updateVisibleTasks() {
      const oneWeekAgo = new Date(Date.now() - 7 * 24 * 60 * 60 * 1000).toISOString().split("T")[0];
      if (hidePastTasks.checked) {
        visibleTasks = tasks.filter((task) => {
          return !task.end || task.end >= oneWeekAgo;
        });
      } else {
        visibleTasks = [...tasks];
      }
      redraw = true;
    }
    hidePastTasks.addEventListener("change", () => {
      updateVisibleTasks();
    });

    const container = document.getElementById("canvas-container") as HTMLDivElement | null;
    if (!container) throw new Error("canvas container not found");
    const canvasWidth = container.clientWidth;
    const canvasHeight = Math.max(p.windowHeight * 0.8, visibleTasks.length * yBoxSpacing + 300); // Padding added at the bottom for aesthetics
    const canvas = p.createCanvas(canvasWidth, canvasHeight);
    canvas.parent("canvas-container");

    let fourMonthsMillis = 12 * 30 * 24 * 60 * 60 * 1000; // Approx 12 months in ms
    zoomLevel = (canvasWidth - startX) / ((fourMonthsMillis / (timelineEnd - timelineStart)) * (canvasWidth - startX));

    // Compute the "Now" position and set initial offset
    let nowTime = new Date().getTime();
    let nowX = dateToX(new Date(nowTime).toISOString().split("T")[0], p);

    // Ensure "Now" starts slightly to the right of the starting position
    offsetX = startX - nowX + 150; // Adjust 150px to give some left padding

    let zoomSlider = document.getElementById("zoom-slider") as HTMLInputElement;
    zoomSlider.value = zoomLevel.toFixed(2);
    zoomSlider.addEventListener("input", () => {
      zoomLevel = parseFloat(zoomSlider.value);
      redraw = true;
      p.redraw();
    });

    // Preset zoom buttons.
    const applyPreset = (preset: "fit" | "next-3m" | "next-6m" | "center-3m" | "center-6m" | "center-12m") => {
      const visibleWidth = p.width - startX;
      const monthMillis = 30 * 24 * 60 * 60 * 1000;
      const totalRange = timelineEnd - timelineStart;
      const nowMillis = new Date().getTime();
      const nowProgress = (nowMillis - timelineStart) / totalRange;
      // Padding behind the "Now" line for forward-looking presets so it
      // doesn't sit right on the left edge.
      const nowLeftPadding = 80;

      let newZoom: number;
      let desiredNowX: number | null = null;

      switch (preset) {
        case "fit":
          newZoom = 1;
          offsetX = 0;
          break;
        case "next-3m":
          newZoom = totalRange / (3 * monthMillis);
          desiredNowX = startX + nowLeftPadding;
          break;
        case "next-6m":
          newZoom = totalRange / (6 * monthMillis);
          desiredNowX = startX + nowLeftPadding;
          break;
        case "center-3m":
          newZoom = totalRange / (6 * monthMillis);
          desiredNowX = startX + visibleWidth / 2;
          break;
        case "center-6m":
          newZoom = totalRange / (12 * monthMillis);
          desiredNowX = startX + visibleWidth / 2;
          break;
        case "center-12m":
          newZoom = totalRange / (24 * monthMillis);
          desiredNowX = startX + visibleWidth / 2;
          break;
      }

      zoomLevel = newZoom;
      if (desiredNowX !== null) {
        offsetX = desiredNowX - startX - nowProgress * visibleWidth * newZoom;
      }

      // Reflect new zoom in the slider (will clamp to slider bounds visually).
      zoomSlider.value = String(newZoom);
      redraw = true;
      p.redraw();
    };

    document.getElementById("zoom-fit-all")?.addEventListener("click", () => applyPreset("fit"));
    document.getElementById("zoom-next-3m")?.addEventListener("click", () => applyPreset("next-3m"));
    document.getElementById("zoom-next-6m")?.addEventListener("click", () => applyPreset("next-6m"));
    document.getElementById("zoom-3m-3m")?.addEventListener("click", () => applyPreset("center-3m"));
    document.getElementById("zoom-6m-6m")?.addEventListener("click", () => applyPreset("center-6m"));
    document.getElementById("zoom-1y-1y")?.addEventListener("click", () => applyPreset("center-12m"));

    p.textFont("Arial");

    console.log("🚀 Initializing Gantt Chart...");

    redraw = true;
    p.frameRate(30);
  };

  p.draw = () => {
    // ---------------- MILESTONE LABEL SPACE CALC ----------------
    let milestoneLevels: { xStart: number; xEnd: number; yLevel: number }[] = [];
    let mileStoneBoxHeight = 20;
    let maxYLevel = 20; // Default top margin
    p.textSize(14);

    visibleTasks.forEach((task) => {
      if (task.milestone) {
        let x = dateToX(task.milestone, p);
        let boxWidth = p.textWidth(task.name) + 10;
        let alignRight = task.milestone === task.end;

        // Determine xStart and xEnd based on alignment
        let xStart = alignRight ? x - boxWidth : x;
        let xEnd = alignRight ? x : x + boxWidth;

        // Find the lowest available level that doesn’t overlap
        let yOffset = 20; // Start at top position
        for (const level of milestoneLevels) {
          if (!(xEnd < level.xStart || xStart > level.xEnd)) {
            yOffset = level.yLevel + mileStoneBoxHeight; // Move down if overlapping
          }
        }

        // Track max vertical space required
        maxYLevel = Math.max(maxYLevel, yOffset);

        // Store this label's occupied space
        milestoneLevels.push({ xStart, xEnd, yLevel: yOffset });
      }
    });

    // Compute final milestone space height
    let milestoneSpaceHeight = maxYLevel;

    let monthTextSize = 14;
    let dayNumberSize = 12;
    let headerPadding = 10;
    timelineTop = monthTextSize + dayNumberSize + milestoneSpaceHeight + headerPadding;
    dateAreaHeight = timelineTop;

    // ---------------- HOVER CURSOR + TOOLTIP DETECTION ----------------
    document.body.style.cursor = "default"; // Reset cursor on each frame

    // Date area hover detection
    let withinCanvas = p.mouseX >= 0 && p.mouseX <= p.width && p.mouseY >= 0 && p.mouseY <= p.height;
    let withinDateArea = p.mouseY <= dateAreaHeight;
    if (withinCanvas && withinDateArea) {
      if (isDragging) {
        document.body.style.cursor = "grabbing";
      } else {
        document.body.style.cursor = "grab";
      }
    }

    // Task hover detection
    let isHovering = false;
    let showToolTip = false;
    visibleTasks.forEach((task, i) => {
      let xStart = dateToX(task.start, p);
      let xEnd = dateToX(task.end, p);
      let y = i * yBoxSpacing + timelineTop;

      let edgePadding = 5; // Hover detection range
      let withinYBounds = p.mouseY >= y && p.mouseY <= y + barHeight;
      if (withinYBounds) {
        if (p.mouseX >= xStart - edgePadding && p.mouseX <= xStart + edgePadding) {
          document.body.style.cursor = "ew-resize"; // Left edge
        } else if (p.mouseX >= xEnd - edgePadding && p.mouseX <= xEnd + edgePadding) {
          document.body.style.cursor = "ew-resize"; // Right edge
        } else if (p.mouseX > xStart + edgePadding && p.mouseX < xEnd - edgePadding) {
          if (isDragging && draggingEdge === "middle") {
            document.body.style.cursor = "grabbing";
          } else {
            document.body.style.cursor = "grab"; // Middle of task
          }
        }
      }

      // Detect mouse hover for tool tip
      isHovering = p.mouseX >= xStart && p.mouseX <= xEnd && p.mouseY >= y && p.mouseY <= y + barHeight;
      if (isHovering) {
        toolTipTask = task;
        redraw = true;
        showToolTip = true;
      }
    });
    // If the tool tip task is set (from the previous draw) but we no longer need to show it, draw once again with no tooltip.
    if (!showToolTip && toolTipTask !== null) {
      redraw = true;
      toolTipTask = null;
    }
    if (!isDragging && !redraw) {
      return;
    }

    // console.log("drawing timeline...");
    p.clear(); // Clears previous frame
    p.textAlign(p.CENTER, p.BOTTOM);
    p.textSize(12);
    p.background(255);

    // ---------------- TIMELINE HEADER ----------------
    p.strokeWeight(1);
    p.stroke(180);
    p.line(0, timelineTop, p.width, timelineTop);

    // Date rendering setup
    let interval = 1 * 24 * 60 * 60 * 1000; // 1 day in milliseconds
    p.textSize(12);
    p.textAlign(p.CENTER);
    p.fill(50);
    let minNumberSpacing = 15; // Minimum spacing to avoid overlap
    let isGrey = false; // Toggle for alternating colors

    // ---------------- VIEWPORT BOUNDS ----------------
    // Inverse of dateToX: only iterate over days that are visible on screen,
    // which decouples the timeline axis rendering from the data range.
    const totalRange = timelineEnd - timelineStart;
    const xToMillisUnclamped = (x: number) => timelineStart + ((x - startX - offsetX) / ((p.width - startX) * zoomLevel)) * totalRange;
    const vpStartMillis = xToMillisUnclamped(0);
    const vpEndMillis = xToMillisUnclamped(p.width);

    // ---------------- DAY-NUMBER STRIDE ----------------
    // Anchor which days get labelled to the date itself (not loop order) so
    // labels don't shimmer between e.g. {2,5,8,...} and {3,6,9,...} as the
    // user pans. Pick a "nice" stride based on pixels-per-day at the current
    // zoom that still respects minNumberSpacing.
    const pixelsPerDay = ((p.width - startX) * zoomLevel * interval) / totalRange;
    const minStride = Math.max(1, Math.ceil(minNumberSpacing / pixelsPerDay));
    const niceStrides = [1, 2, 3, 5, 7, 14, 30];
    const dayStride = niceStrides.find((s) => s >= minStride) ?? Math.ceil(minStride / 30) * 30;

    // Snap loop start to start-of-day so day numbers/lines are pixel-stable.
    const loopStartDate = new Date(vpStartMillis);
    loopStartDate.setHours(0, 0, 0, 0);
    const dayLoopStart = loopStartDate.getTime();

    // First visible month's start (used for label/divider positioning of the
    // partially-visible left-edge month).
    const firstMonthStartDate = new Date(loopStartDate);
    firstMonthStartDate.setDate(1);
    const firstMonthStartX = dateToX(firstMonthStartDate.toISOString().split("T")[0], p);
    const firstMonthYear = firstMonthStartDate.toLocaleString("default", { month: "long", year: "numeric" });

    // Pre-draw the first visible month's label, since the loop starts inside
    // that month and won't see a month transition for it.
    p.fill(0);
    p.strokeWeight(0);
    p.textSize(14);
    p.text(firstMonthYear, firstMonthStartX + 15, monthTextSize + 2);

    let lastMonthX = firstMonthStartX;
    let lastMonth: number = firstMonthStartDate.getMonth();

    for (let t = dayLoopStart; t <= vpEndMillis; t += interval) {
      let date = new Date(t);
      let x = dateToX(date.toISOString().split("T")[0], p);
      let month: number = date.getMonth();

      let day = date.getDate();
      let monthYear = date.toLocaleString("default", { month: "long", year: "numeric" });

      // ---------------- MONTH + YEAR ----------------
      if (month !== lastMonth) {
        p.fill(0);
        p.strokeWeight(0);
        p.textSize(14);
        p.text(monthYear, x + 15, monthTextSize + 2);

        // Draw divider at the previous month's start.
        p.stroke(150);
        p.strokeWeight(1);
        p.line(lastMonthX, monthTextSize + 10, lastMonthX, p.height - 10);
        isGrey = !isGrey; // Toggle color for next month

        // Track new month's start position
        lastMonthX = x;
        lastMonth = month;
      }

      // ---------------- DAY NUMBER ----------------
      // Label only days that fall on the date-anchored stride, so the shown
      // day numbers are stable as the user pans.
      const dayIndex = Math.floor(t / interval);
      if (dayIndex % dayStride === 0) {
        p.fill(0);
        p.strokeWeight(0);
        p.textSize(dayNumberSize);
        p.text(day, x, monthTextSize + dayNumberSize + milestoneSpaceHeight + 8);
      }

      // ---------------- WEEKEND SHADING ----------------
      let isWeekend = date.getDay() === 0 || date.getDay() === 6;
      if (isWeekend) {
        p.fill(248, 248, 248);
        p.noStroke();
        let nextX = dateToX(new Date(t + interval).toISOString().split("T")[0], p);
        let width = nextX - x;
        p.rect(x, timelineTop, width, p.height - 60);
      }

      // ---------------- DAILY VERTICAL LINES ----------------
      p.stroke(220);
      p.strokeWeight(1);
      p.line(x, timelineTop, x, p.height - 10);
    }

    // ---------------- FINAL MONTH LINE ----------------
    p.stroke(150);
    p.strokeWeight(1);
    p.line(lastMonthX, monthTextSize + 10, lastMonthX, p.height - 10);

    // ---------------- BACKGROUND COLOUR FOR OWNER ----------------
    visibleTasks.forEach((task, index) => {
      let yPos = index * yBoxSpacing + timelineTop + 5;
      let backgroundColor = getOwnerColor(task.owner);

      // Draw background color for each row.
      p.fill(backgroundColor);
      p.noStroke();
      p.rect(0, yPos - 5, p.width, yBoxSpacing);
    });

    // ---------------- VERTICAL MILESTONE LINES AND TITLES ----------------
    milestoneLevels = [];

    p.strokeWeight(2);
    p.textSize(14);
    visibleTasks.forEach((task) => {
      if (task.milestone) {
        let x = dateToX(task.milestone, p);

        let boxColor = hexToP5Color(getPriorityColor(task.priority), 0.8, p);
        let boxWidth = p.textWidth(task.name) + 10;
        let alignRight = task.milestone === task.end;

        // Determine xStart and xEnd based on alignment
        let xStart = alignRight ? x - boxWidth : x;
        let xEnd = alignRight ? x : x + boxWidth;

        // Find the lowest available level that doesn’t overlap
        let yOffset = 20; // Start at top position
        for (const level of milestoneLevels) {
          if (!(xEnd < level.xStart || xStart > level.xEnd)) {
            yOffset = level.yLevel + 20; // Move down if overlapping
          }
        }

        // Store this label's occupied space
        milestoneLevels.push({ xStart, xEnd, yLevel: yOffset });

        // Draw milestone line
        p.stroke(150, 0, 255);
        p.line(x, yOffset, x, p.height - 10);

        // Draw milestone label
        p.fill(boxColor);
        p.rect(xStart, yOffset - mileStoneBoxHeight + monthTextSize, boxWidth, mileStoneBoxHeight, 3); // Rounded corners

        // Correct text alignment inside the box
        p.fill(0);
        p.noStroke();
        p.textAlign(alignRight ? p.RIGHT : p.LEFT);
        p.text(task.name, alignRight ? x - 5 : x + 5, yOffset + monthTextSize - 2);
      }
    });

    visibleTasks.forEach((task, i) => {
      let xStart = dateToX(task.start, p);
      let y = i * yBoxSpacing + timelineTop;
      // ---------------- DRAW DEPENDENCY ARROWS ----------------
      task.dependencies.forEach((depID) => {
        let dependencyTask = visibleTasks.find((t) => t.id === depID);
        if (dependencyTask) {
          let xDepEnd = dateToX(dependencyTask.end, p); // Pointing to dependency's end
          let yDep = visibleTasks.indexOf(dependencyTask) * yBoxSpacing + timelineTop;

          drawArrow(p, xStart, y + barHeight / 2, xDepEnd, yDep + barHeight / 2);
        }
      });
    });

    // ---------------- TASK BOXES ----------------
    p.textSize(14);
    visibleTasks.forEach((task, i) => {
      let xStart = dateToX(task.start, p);
      let xEnd = dateToX(task.end, p);
      let y = i * yBoxSpacing + timelineTop;

      p.fill(getPriorityColor(task.priority));
      p.rect(xStart, y, xEnd - xStart, barHeight, 5);

      let textWidth = p.textWidth(task.name);
      let padding = 6; // Extra space around text
      let boxHeight = 18; // Background height

      p.fill(255, 150); // White background
      p.noStroke();
      p.rect(xStart + 5 - padding / 2, y + barHeight / 2 - boxHeight / 2, textWidth + padding, boxHeight, 3); // Rounded corners

      p.fill(0);
      p.strokeWeight(0);
      p.textAlign(p.LEFT, p.CENTER);
      p.text(task.name, xStart + 5, y + barHeight / 2);
    });

    // ---------------- NOW LINE ----------------
    let nowTime = new Date();
    nowX = dateToX(nowTime.toISOString().split("T")[0], p);

    // Now line.
    p.stroke(255, 0, 0);
    p.strokeWeight(2);
    p.line(nowX, timelineTop - dayNumberSize - headerPadding, nowX, p.height - 10);

    // Now label.
    let boxWidth = p.textWidth("Now") + 10;
    let boxHeight = 18;
    p.fill(255);
    p.rect(nowX - boxWidth / 2, timelineTop - dayNumberSize - boxHeight - headerPadding, boxWidth, boxHeight, 3); // Rounded corners
    p.fill(255, 0, 0);
    p.noStroke();
    p.textSize(14);
    p.textAlign(p.CENTER);
    p.text("Now", nowX, timelineTop - boxHeight - headerPadding - 2);

    // ---------------- LABEL FOR OWNER ----------------
    let currentOwner = "";
    visibleTasks.forEach((task, index) => {
      // Only draw Owner name when it changes (first occurrence).
      let yPos = index * yBoxSpacing + timelineTop + 5;
      if (task.owner !== currentOwner) {
        currentOwner = task.owner;
        let firstName = currentOwner.split(" ")[0];
        let textWidth = p.textWidth(firstName);
        let padding = 6; // Extra padding around text
        let boxHeight = 18; // Box height
        p.fill(255); // White background
        p.stroke(50);
        p.rect(5 - padding / 2, yPos + barHeight / 2 - boxHeight / 2, textWidth + padding, boxHeight, 3); // Rounded corners

        p.noStroke();
        p.fill(50); // Darker text color.
        p.textSize(14);
        p.textAlign(p.LEFT);
        p.text(firstName, 5, yPos + barHeight / 2); // Left-aligned.
      }
    });

    // ---------------- TASK TOOL TIP ----------------
    visibleTasks.forEach((task, i) => {
      if (showToolTip && toolTipTask === task) {
        drawTooltip(p, task, p.mouseX, p.mouseY);
      }
    });

    redraw = false;
  };

  // Enable panning with mouse drag
  p.mousePressed = () => {
    dateAreaHeight = timelineTop;
    isDragging = true;
    dragStartX = p.mouseX;

    // Latch whether this drag started in the timeline header so that panning
    // keeps tracking even if the cursor strays off the (thin) header strip
    // mid-drag.
    isPanningHeader = p.mouseY >= 0 && p.mouseY <= dateAreaHeight && p.mouseX >= 0 && p.mouseX <= p.width;

    selectedTask = null;
    draggingEdge = null;

    visibleTasks.forEach((task) => {
      let xStart = dateToX(task.start, p);
      let xEnd = dateToX(task.end, p);
      let y = visibleTasks.indexOf(task) * yBoxSpacing + timelineTop;
      let edgePadding = 5;

      if (p.mouseX >= xStart - edgePadding && p.mouseX <= xStart + edgePadding && p.mouseY >= y && p.mouseY <= y + barHeight) {
        draggingEdge = "start";
        selectedTask = task;
      } else if (p.mouseX >= xEnd - edgePadding && p.mouseX <= xEnd + edgePadding && p.mouseY >= y && p.mouseY <= y + barHeight) {
        draggingEdge = "end";
        selectedTask = task;
      } else if (p.mouseX > xStart + edgePadding && p.mouseX < xEnd - edgePadding && p.mouseY >= y && p.mouseY <= y + barHeight) {
        draggingEdge = "middle";
        selectedTask = task;
      }
    });
  };

  p.mouseReleased = () => {
    isDragging = false;
    isPanningHeader = false;
    accumulatedDeltaMillis = 0;

    // If the user just dragged a task (possibly past the previous data
    // range), recompute timelineStart/End and re-derive zoomLevel/offsetX so
    // the visible pixel positions stay identical. This keeps "Fit all" and
    // the other zoom presets honest about the new data range without
    // visually disturbing the user's current view.
    if (selectedTask && tasks.length > 0) {
      const W = p.width - startX;
      const oldRange = timelineEnd - timelineStart;

      // Date currently at x=0; we'll pin it there after re-parameterizing.
      const oldLeftMillis = timelineStart + ((0 - startX - offsetX) / (W * zoomLevel)) * oldRange;

      const newStart = Math.min(...tasks.map((t) => new Date(t.start).getTime()));
      const newEnd = Math.max(...tasks.map((t) => new Date(t.end).getTime()));
      const newRange = newEnd - newStart;

      if (oldRange > 0 && newRange > 0) {
        timelineStart = newStart;
        timelineEnd = newEnd;
        // dateToX is affine in date; matching at one point + scaling the
        // slope by newRange/oldRange ensures it matches everywhere.
        zoomLevel = (zoomLevel * newRange) / oldRange;
        offsetX = -startX - ((oldLeftMillis - timelineStart) / newRange) * W * zoomLevel;

        const slider = document.getElementById("zoom-slider") as HTMLInputElement | null;
        if (slider) slider.value = String(zoomLevel);
        redraw = true;
      }
    }
  };

  let draggingEdge: "start" | "end" | "middle" | null = null;
  let selectedTask: any = null;
  let dateAreaHeight = 50; // Adjust as needed
  let accumulatedDeltaMillis = 0;
  let isPanningHeader = false;

  p.mouseDragged = () => {
    // Once a header pan has started, keep tracking horizontal movement until
    // mouseup, regardless of where the cursor wanders. The header strip is
    // narrow and easy to drift off vertically.
    if (isDragging && isPanningHeader) {
      let movement = p.mouseX - p.pmouseX;
      offsetX += movement;
    }

    if (!isDragging || !selectedTask) return;

    let newDate = xToDate(p.mouseX, p);
    if (draggingEdge === "start") {
      selectedTask.start = newDate;
    } else if (draggingEdge === "end") {
      selectedTask.end = newDate;
    } else if (draggingEdge === "middle") {
      let deltaPixels = p.mouseX - p.pmouseX;
      let timeRange = timelineEnd - timelineStart;
      let millisPerPixel = timeRange / ((p.width - startX) * zoomLevel);
      let deltaMillis = deltaPixels * millisPerPixel;

      accumulatedDeltaMillis += deltaMillis; // ✅ Accumulate movement

      const oneDayMillis = 24 * 60 * 60 * 1000;

      // ✅ Only if we have moved a full day (or more)
      if (Math.abs(accumulatedDeltaMillis) >= oneDayMillis) {
        const dayChange = Math.round(accumulatedDeltaMillis / oneDayMillis);

        let startMillis = new Date(selectedTask.start).getTime() + dayChange * oneDayMillis;
        let endMillis = new Date(selectedTask.end).getTime() + dayChange * oneDayMillis;

        selectedTask.start = new Date(startMillis).toISOString().split("T")[0];
        selectedTask.end = new Date(endMillis).toISOString().split("T")[0];

        accumulatedDeltaMillis -= dayChange * oneDayMillis; // ✅ Remove committed movement
      }
    }
  };
};

new p5(sketch);

function dateToX(dateStr: string, p: p5): number {
  if (!dateStr) return startX; // Prevents crashes

  let dateObj = new Date(dateStr);
  if (isNaN(dateObj.getTime())) {
    console.error(`❌ Invalid date detected: ${dateStr}`);
    return startX; // Prevents drawing errors
  }

  let dateMillis = dateObj.getTime();
  let progress = (dateMillis - timelineStart) / (timelineEnd - timelineStart);
  return startX + progress * (p.width - startX) * zoomLevel + offsetX;
}

function xToDate(x: number, p: p5): string {
  let timeRange = timelineEnd - timelineStart;

  // Adjust X position for offset & zoom
  let adjustedX = (x - startX - offsetX) / zoomLevel;

  // Map X position to a date. Intentionally not clamped to the data range so
  // tasks can be dragged past the first/last task into open future or past.
  let dateMillis = timelineStart + (adjustedX / (p.width - startX)) * timeRange;

  return new Date(dateMillis).toISOString().split("T")[0]; // Format as YYYY-MM-DD
}

function hexToP5Color(hex: string, alpha: number, p: p5) {
  let col = p.color(hex); // Convert hex to p5 color
  col.setAlpha(alpha * 255); // p5.js alpha is 0-255, so multiply by 255
  return col;
}

function drawArrow(p: p5, x1: number, y1: number, x2: number, y2: number) {
  p.stroke(100);
  p.strokeWeight(2);
  p.noFill();

  let midX = (x1 + x2) / 2;
  let midY = (y1 + y2) / 2;

  // Draw a curved line (Bezier)
  p.beginShape();
  p.vertex(x2, y2); // Swapped start & end
  p.bezierVertex(midX, y2, midX, y1, x1, y1); // Flipped Bezier direction
  p.endShape();

  // Move the arrowhead to `x1, y1` (flipped direction)
  let arrowSize = 6;
  let angle = Math.atan2(y1 - y2, x1 - x2); // Flip angle calculation
  p.fill(100);
  p.noStroke();
  p.triangle(
    x1,
    y1, // Arrowhead now correctly at the **start** of the line
    x1 - arrowSize * Math.cos(angle + Math.PI / 6),
    y1 - arrowSize * Math.sin(angle + Math.PI / 6),
    x1 - arrowSize * Math.cos(angle - Math.PI / 6),
    y1 - arrowSize * Math.sin(angle - Math.PI / 6),
  );
}

function drawTooltip(p: p5, task: any, x: number, y: number) {
  const padding = 8;
  const title = task.name || "(No Title)";
  const description = task.description || "(No Description)";
  const category = task.category || "";

  // Set text properties
  p.textSize(14);
  p.textAlign(p.LEFT, p.TOP);

  const titleWidth = p.textWidth(title);
  p.textSize(12);
  const descWidth = p.textWidth(description);
  const categoryWidth = p.textWidth(category);

  const boxWidth = Math.max(titleWidth, descWidth, categoryWidth) + padding * 2;
  const boxHeight = 65; // Adjusted height to fit 3 lines (title, description, category)

  // Keep tooltip inside canvas
  if (x + boxWidth > p.width) {
    x = p.width - boxWidth - 10;
  }
  if (y + boxHeight > p.height) {
    y = p.height - boxHeight - 10;
  }

  // Draw background box
  p.fill(255);
  p.stroke(0);
  p.strokeWeight(1);
  p.rect(x, y, boxWidth, boxHeight, 5); // Rounded corners

  // Draw text
  p.noStroke();
  p.fill(0);
  p.textSize(14);
  p.text(title, x + padding, y + padding);
  p.textSize(12);
  p.text(description, x + padding, y + padding + 20);
  p.text(`Category: ${category}`, x + padding, y + padding + 38); // ✅ Category line
}

document.getElementById("submit-changes")!.addEventListener("click", async () => {
  try {
    await submitTasks(tasks);
  } catch (error) {
    console.error("❌ Error submitting changes:", error);
  }
});
