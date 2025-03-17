import { time } from "console";
import p5 from "p5";

let tasks: any[] = [];

let timelineStart: number;
let timelineEnd: number;
let startX = 100; // Left margin
let barHeight = 30;
let yBoxSpacing = 35;
let offsetX = 0; // Used for panning
let isDragging = false;
let dragStartX = 0;
let nowX = 0;
let redraw = false;
let zoomLevel = 1; // Default zoom level
let timelineTop = 0; // Updated in draw

// p5.js sketch
const sketch = (p: p5) => {
    p.setup = async () => {
        await fetchGanttData();
        const canvasWidth = p.windowWidth * 0.9;
        const canvasHeight = Math.max(p.windowHeight * 0.8, tasks.length * yBoxSpacing + 300);
        const canvas = p.createCanvas(canvasWidth, canvasHeight);
        canvas.parent("canvas-container");

        let fourMonthsMillis = 4 * 30 * 24 * 60 * 60 * 1000; // Approx 4 months in ms
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
    
        timelineStart = Math.min(...tasks.map(t => new Date(t.start).getTime()));
        timelineEnd = Math.max(...tasks.map(t => new Date(t.end).getTime()));
        redraw = true;
        p.frameRate(30);
    };
    
    p.draw = () => {
        // ---------------- MILESTONE LABEL SPACE CALC ----------------
        let milestoneLevels: { xStart: number; xEnd: number; yLevel: number }[] = [];
        let mileStoneBoxHeight = 20;
        let maxYLevel = 20; // Default top margin

        tasks.forEach(task => {
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

        // ---------------- HOVER CURSOR CALC ----------------
        document.body.style.cursor = "default"; // Reset cursor on each frame
        tasks.forEach((task, i) => {
            let xStart = dateToX(task.start, p);
            let xEnd = dateToX(task.end, p);
            let y = i * yBoxSpacing + timelineTop;

            let edgePadding = 5; // Hover detection range
            let withinYBounds = p.mouseY >= y && p.mouseY <= y + barHeight;
            if (withinYBounds && (
                (p.mouseX >= xStart - edgePadding && p.mouseX <= xStart + edgePadding) ||
                (p.mouseX >= xEnd - edgePadding && p.mouseX <= xEnd + edgePadding)
            )) {
                document.body.style.cursor = "ew-resize";
            }
        });
        if (!isDragging && !redraw) {
            return;
        }

        console.log("drawing timeline...");
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
                if (lastMonth !== null) { // Skip first iteration
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
            if (x - lastRenderedDayX >= minNumberSpacing) { // Ensure minimum spacing
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
        tasks.forEach((task, index) => {
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
        tasks.forEach(task => {
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
        
        tasks.forEach((task, i) => {
            let xStart = dateToX(task.start, p);
            let y = i * yBoxSpacing + timelineTop;
            // ---------------- DRAW DEPENDENCY ARROWS ----------------
            task.dependencies.forEach(depID => {
                let dependencyTask = tasks.find(t => t.id === depID);
                if (dependencyTask) {
                    let xDepEnd = dateToX(dependencyTask.end, p); // ✅ Now pointing **to** dependency's end
                    let yDep = tasks.indexOf(dependencyTask) * yBoxSpacing + timelineTop;
        
                    drawArrow(p, xStart, y + barHeight / 2, xDepEnd, yDep + barHeight / 2);
                }
            });
        });

        // ---------------- TASK BOXES ----------------
        tasks.forEach((task, i) => {
            let xStart = dateToX(task.start, p);
            let xEnd = dateToX(task.end, p);
            let y = i * yBoxSpacing + timelineTop;

            p.fill(getPriorityColor(task.priority));
            p.rect(xStart, y, xEnd - xStart, barHeight, 5);


            let textWidth = p.textWidth(task.name);
            let padding = 6;  // Extra space around text
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
        tasks.forEach((task, index) => {
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

        redraw = false;
    };
    
    // Enable panning with mouse drag
    p.mousePressed = () => {
        dateAreaHeight = timelineTop;
        isDragging = true;

        // Just store the current mouse position directly
        dragStartX = p.mouseX;
    
        selectedTask = null;
        draggingEdge = null;
    
        tasks.forEach((task) => {
            let xStart = dateToX(task.start, p);
            let xEnd = dateToX(task.end, p);
            let y = tasks.indexOf(task) * yBoxSpacing + timelineTop;
            let edgePadding = 5;
    
            if (p.mouseX >= xStart - edgePadding && p.mouseX <= xStart + edgePadding && p.mouseY >= y && p.mouseY <= y + barHeight) {
                draggingEdge = "start";
                selectedTask = task;
            } else if (p.mouseX >= xEnd - edgePadding && p.mouseX <= xEnd + edgePadding && p.mouseY >= y && p.mouseY <= y + barHeight) {
                draggingEdge = "end";
                selectedTask = task;
            }
        });
    };    

    p.mouseReleased = () => {
        isDragging = false;
    };

    let draggingEdge: "start" | "end" | null = null;
    let selectedTask: any = null;
    let dateAreaHeight = 50; // Adjust as needed

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
        }
    };    
};

new p5(sketch);

function dateToX(dateStr: string, p: p5): number {
    let dateMillis = new Date(dateStr).getTime();
    let progress = (dateMillis - timelineStart) / (timelineEnd - timelineStart);
    return startX + (progress * (p.width - startX) * zoomLevel) + offsetX;
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
        x1, y1, // Arrowhead now correctly at the **start** of the line
        x1 - arrowSize * Math.cos(angle + Math.PI / 6), y1 - arrowSize * Math.sin(angle + Math.PI / 6),
        x1 - arrowSize * Math.cos(angle - Math.PI / 6), y1 - arrowSize * Math.sin(angle - Math.PI / 6)
    );
}

async function fetchGanttData() {
    try {
        console.log("🔄 Fetching Gantt data from API...");

        const response = await fetch("http://localhost:8080/timeline");
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
                    name: `${categoryEmoji} ${row.Title}`, // Prepend emoji to task name.
                    start: startDate,
                    end: endDate,
                    priority: row.Priority || "P5",
                    owner: row.Owner || "Other",
                    milestone: row["Milestone Type"] === "Start Date" ? startDate :
                           row["Milestone Type"] === "End Date" ? endDate : null,
                    dependencies: row.Dependencies ? row.Dependencies.split(",").map(dep => dep.trim()) : []
                };
        });

        console.log("🛠️ Processed Tasks:", tasks);

        // Ensure timeline range is updated.
        if (tasks.length > 0) {
            timelineStart = Math.min(...tasks.map(t => new Date(t.start).getTime()));
            timelineEnd = Math.max(...tasks.map(t => new Date(t.end).getTime()));
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
    "Other": "rgba(240, 240, 240, 0.3)" // Default Gray
};

// Color mapping for priority
function getPriorityColor(priority: string): string {
    switch (priority) {
        case "P0": return "#f87171";  // Lighter Red
        case "P1": return "#fb923c";  // Lighter Orange
        case "P2": return "#facc15";  // Lighter Yellow
        case "P3": return "#4ade80";  // Lighter Green
        case "P4": return "#93c5fd";  // Lighter Blue
        case "P5": return "#bfdbfe";  // Very Light Blue
        default: return "#f3f4f6";    // Lighter Gray
    }
}

function getCategoryEmoji(category: string): string {
    const categoryMap: Record<string, string> = {
        "Rig": "🛰️",
        "AusOceanTV Platform": "📺",
        "Hydrophone": "🎤",
        "Camera": "📷",
        "Broadcast": "🎥",
        "CloudBlue": "☁️",
        "Speaker": "🔊",
        "OpenFish": "🐟",
        "Jetty Rig": "🏗️",
        "Other": "⚙️",
    };

    return categoryMap[category] || "⚙️"; // Default to "Other" emoji if no match
}

document.getElementById("submit-changes")!.addEventListener("click", async () => {
    console.log("📤 Sending update request...", tasks);

    try {
        const response = await fetch("http://localhost:8080/update", {
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
