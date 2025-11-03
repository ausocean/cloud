/// <reference types="vite/client" />

import p5 from "p5";

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
    await fetchGanttData();

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
      p.redraw();
    });

    p.textFont("Arial");

    console.log("🚀 Initializing Gantt Chart...");

    timelineStart = Math.min(...visibleTasks.map((t) => new Date(t.start).getTime()));
    timelineEnd = Math.max(...visibleTasks.map((t) => new Date(t.end).getTime()));
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
    p.line(startX + offsetX, timelineTop, p.width - startX + offsetX, timelineTop);

    // Date rendering setup
    let interval = 1 * 24 * 60 * 60 * 1000; // 1 day in milliseconds
    p.textSize(12);
    p.textAlign(p.CENTER);
    p.fill(50);
    let lastRenderedDayX = -Infinity; // Track last rendered day position
    let minNumberSpacing = 15; // Minimum spacing to avoid overlap
    let lastMonthX = dateToX(new Date(timelineStart).toISOString().split("T")[0], p);
    let lastMonth: number | null = null;
    let isGrey = false; // Toggle for alternating colors
    for (let t = timelineStart; t <= timelineEnd; t += interval) {
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
        if (lastMonth !== null) {
          // Skip first iteration
          p.stroke(150);
          p.strokeWeight(1);
          p.line(lastMonthX, monthTextSize + 10, lastMonthX, p.height - 10);
          isGrey = !isGrey; // Toggle color for next month
        }

        // Track new month's start position
        lastMonthX = x;
        lastMonth = month;
      }

      // ---------------- DAY NUMBER ----------------
      if (x - lastRenderedDayX >= minNumberSpacing) {
        // Ensure minimum spacing
        p.fill(0);
        p.strokeWeight(0);
        p.textSize(dayNumberSize);
        p.text(day, x, monthTextSize + dayNumberSize + milestoneSpaceHeight + 8);
        lastRenderedDayX = x; // Update last rendered position
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
    if (lastMonth !== null) {
      p.stroke(150);
      p.strokeWeight(1);
      p.line(lastMonthX, monthTextSize + 10, lastMonthX, p.height - 10);
    }

    // ---------------- BACKGROUND COLOUR FOR OWNER ----------------
    visibleTasks.forEach((task, index) => {
      let yPos = index * yBoxSpacing + timelineTop + 5;
      let backgroundColor = ownerColors[task.owner] || ownerColors["Other"];

      // Draw background color for each row.
      p.fill(backgroundColor);
      p.noStroke();
      p.rect(0, yPos - 5, p.width, yBoxSpacing);
    });

    // ---------------- VERTICAL MILESTONE LINES AND TITLES ----------------
    milestoneLevels = [];

    p.strokeWeight(2);
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
      p.textSize(14);
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
    accumulatedDeltaMillis = 0;
  };

  let draggingEdge: "start" | "end" | "middle" | null = null;
  let selectedTask: any = null;
  let dateAreaHeight = 50; // Adjust as needed
  let accumulatedDeltaMillis = 0;

  p.mouseDragged = () => {
    let canvasWidth = p.width;
    let canvasHeight = p.height;

    let withinCanvas = p.mouseX >= 0 && p.mouseX <= canvasWidth && p.mouseY >= 0 && p.mouseY <= canvasHeight;
    let withinDateArea = p.mouseY <= dateAreaHeight;

    if (isDragging && withinCanvas && withinDateArea) {
      // Calculate the movement since the last frame
      let movement = p.mouseX - p.pmouseX;

      // Update offsetX by adding the movement
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

  // Ensure proper mapping of X position to time range
  let dateMillis = timelineStart + (adjustedX / (p.width - startX)) * timeRange;

  // Prevent invalid dates due to out-of-bounds values
  dateMillis = Math.max(timelineStart, Math.min(dateMillis, timelineEnd));

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

const API_BASE_URL = import.meta.env.VITE_API_URL || "http://localhost:8080";

async function fetchGanttData() {
  try {
    console.log("🔄 Fetching Gantt data from API...");

    const response = await fetch(`${API_BASE_URL}/api/v1/timeline`);
    console.log("API Response Received:", response);

    if (!response.ok) {
      throw new Error(`HTTP error! Status: ${response.status}`);
    }

    const rawData = await response.json();
    console.log("📄 Raw API Data:", rawData);

    // Convert API response to Task format with date parsing.
    tasks = rawData
      .filter((row: any) => row.Status !== "Discontinued")
      .map((row: any) => {
        let startDate = parseDate(row.ActualStart || row.Start || "");
        let endDate = parseDate(row.ActualEnd || row.End || "");
        let categoryEmoji = getCategoryEmoji(row.Category || "Other");

        // Validate start and end dates.
        if (!startDate || isNaN(new Date(startDate).getTime())) {
          console.warn(`⚠️ Invalid start date for task "${row.Title}":`, startDate);
          startDate = new Date().toISOString().split("T")[0]; // Default to today.
        }

        if (!endDate || isNaN(new Date(endDate).getTime())) {
          console.warn(`⚠️ Invalid end date for task "${row.Title}":`, endDate);
          endDate = new Date(new Date(startDate).getTime() + 7 * 24 * 60 * 60 * 1000).toISOString().split("T")[0]; // Default to 7 days later
        }

        return {
          id: row.ID,
          category: row.Category,
          name: `${categoryEmoji} ${row.Title}`, // Prepend emoji to task name.
          description: row.Description || "",
          start: startDate,
          end: endDate,
          priority: row.Priority || "P5",
          owner: row.Owner || "Other",
          milestone: row["Milestone Type"] === "Start Date" ? startDate : row["Milestone Type"] === "End Date" ? endDate : null,
          dependencies: row.Dependencies ? row.Dependencies.split(",").map((dep) => dep.trim()) : [],
        };
      });

    console.log("🛠️ Processed Tasks:", tasks);

    // Ensure timeline range is updated.
    if (tasks.length > 0) {
      timelineStart = Math.min(...tasks.map((t) => new Date(t.start).getTime()));
      timelineEnd = Math.max(...tasks.map((t) => new Date(t.end).getTime()));
      console.log(`📆 Timeline Updated: Start - ${new Date(timelineStart).toISOString()}, End - ${new Date(timelineEnd).toISOString()}`);
    } else {
      console.warn("⚠️ No tasks found in API response.");
    }
  } catch (error) {
    console.error("❌ Error fetching Gantt data:", error);
  }
}

function parseDate(dateString: string): string {
  const parts = dateString.split("/");
  if (parts.length === 3) {
    // Convert dd/mm/yyyy → yyyy-mm-dd
    return `${parts[2]}-${parts[1].padStart(2, "0")}-${parts[0].padStart(2, "0")}`;
  }
  console.warn(`⚠️ Unexpected date format: "${dateString}"`);
  return ""; // Return empty if the format is incorrect
}

const ownerColors: Record<string, string> = {
  "David Sutton": "rgba(255, 220, 220, 0.3)", // Light Red
  "Saxon Nelson-Milton": "rgba(220, 255, 220, 0.3)", // Light Green
  "Breeze del West": "rgba(220, 220, 255, 0.3)", // Light Blue
  "Trek Hopton": "rgba(255, 255, 220, 0.3)", // Light Yellow
  "Scott Barnard": "rgba(255, 220, 255, 0.3)", // Light Pink
  "Intern-Software": "rgba(255, 220, 190, 0.3)",
  "Intern-Elec": "rgba(190, 220, 190, 0.3)",
  "Intern-Mech": "rgba(255, 190, 255, 0.3)",
  Other: "rgba(240, 240, 240, 0.3)", // Default Gray
};

// Color mapping for priority
function getPriorityColor(priority: string): string {
  switch (priority) {
    case "P0":
      return "#f87171"; // Lighter Red
    case "P1":
      return "#fb923c"; // Lighter Orange
    case "P2":
      return "#facc15"; // Lighter Yellow
    case "P3":
      return "#4ade80"; // Lighter Green
    case "P4":
      return "#93c5fd"; // Lighter Blue
    case "P5":
      return "#bfdbfe"; // Very Light Blue
    default:
      return "#f3f4f6"; // Lighter Gray
  }
}

function getCategoryEmoji(category: string): string {
  const categoryMap: Record<string, string> = {
    Rig: "🛰️",
    "AusOceanTV Platform": "📺",
    Hydrophone: "🎤",
    Camera: "📷",
    Broadcast: "🎥",
    CloudBlue: "☁️",
    Speaker: "🔊",
    OpenFish: "🐟",
    "Jetty Rig": "🏗️",
    Other: "⚙️",
  };

  return categoryMap[category] || "⚙️"; // Default to "Other" emoji if no match
}

document.getElementById("submit-changes")!.addEventListener("click", async () => {
  console.log("📤 Sending update request...", tasks);

  try {
    const response = await fetch(`${API_BASE_URL}/api/v1/update`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ tasks }),
    });

    if (!response.ok) {
      const errorText = await response.text();
      throw new Error(`Failed to update tasks: ${errorText}`);
    }

    console.log("Changes submitted successfully!");
  } catch (error) {
    console.error("❌ Error submitting changes:", error);
  }
});
