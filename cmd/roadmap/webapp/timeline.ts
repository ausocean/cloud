import p5 from "p5";

// Task data structure
interface Task {
    id: string;
    name: string;
    start: string;
    end: string;
    priority: string;
}

let tasks: any[] = [];
let originalTasks: any[] = [];
let modifiedTasks: Set<string> = new Set(); // Track modified task IDs

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


let timelineStart: number;
let timelineEnd: number;
let startX = 100; // Left margin
let barHeight = 30;
let ySpacing = 35;
let offsetX = 0; // Used for panning
let isDragging = false;
let dragStartX = 0;
let nowX = 0;

// Get today's date timestamp
const now = new Date().getTime();

// Convert date to X position
function dateToX(date: string, p: p5): number {
    let dateObj = new Date(date);
    return p.map(dateObj.getTime(), timelineStart, timelineEnd, startX, p.width - startX);
}

// p5.js sketch
const sketch = (p: p5) => {
    p.setup = async () => {
        p.noLoop();
        await fetchGanttData();
        const canvasHeight = Math.max(p.windowHeight * 0.8, tasks.length * ySpacing + 100);
        const canvas = p.createCanvas(p.windowWidth * 0.9, canvasHeight);
        canvas.parent("canvas-container");
        p.textFont("Arial");

        console.log("🚀 Initializing Gantt Chart...");
    
        timelineStart = Math.min(...tasks.map(t => new Date(t.start).getTime()));
        timelineEnd = Math.max(...tasks.map(t => new Date(t.end).getTime()));
        p.redraw();
    };
    
    p.draw = () => {
        p.background(255);
    
        // Draw the timeline axis
        p.strokeWeight(1);
        p.stroke(180);
        p.line(startX + offsetX, 50, p.width - startX + offsetX, 50); // Shifted down for two rows

        // Date rendering setup
        let interval = 1 * 24 * 60 * 60 * 1000; // 1 day in milliseconds
        p.textSize(12);
        p.textAlign(p.CENTER);
        p.fill(50);
        let lastMonthYear = "";

        for (let t = timelineStart; t <= timelineEnd; t += interval) {
            let date = new Date(t);
            let x = dateToX(date.toISOString().split("T")[0], p) + offsetX;

            let day = date.getDate();
            let monthYear = date.toLocaleString("default", { month: "long", year: "numeric" });

            // Draw Month + Year only when it changes.
            if (monthYear !== lastMonthYear) {
                p.fill(0);
                p.strokeWeight(0);
                p.textSize(14);
                p.text(monthYear, x + 15, 15); // Positioned higher
                lastMonthYear = monthYear;
            }

            // Draw the day number.
            p.fill(0);
            p.strokeWeight(0);
            p.textSize(12);
            p.text(day, x, 35); // Below Month/Year.

            let isWeekend = date.getDay() === 0 || date.getDay() === 6;

            if (isWeekend) {
                p.fill(248, 248, 248);
                p.noStroke();
                let nextX = dateToX(new Date(t + interval).toISOString().split("T")[0], p) + offsetX;
                let width = nextX - x;
                p.rect(x, 50, width, p.height - 60);
            }

            // Draw subtle daily grid lines.
            p.stroke(220);
            p.strokeWeight(1);
            p.line(x, 50, x, p.height - 10);
        }
    
        document.body.style.cursor = "default"; // Reset cursor on each frame

        tasks.forEach((task, i) => {
            let xStart = dateToX(task.start, p) + offsetX;
            let xEnd = dateToX(task.end, p) + offsetX;
            let y = i * ySpacing + 50;

            p.fill(getPriorityColor(task.priority));
            p.rect(xStart, y, xEnd - xStart, barHeight, 5);
            p.fill(0);
            p.strokeWeight(0);
            p.textAlign(p.LEFT, p.CENTER);
            p.textSize(14);
            p.text(task.name, xStart + 5, y + barHeight / 2);

            let edgePadding = 5; // Hover detection range
            let withinYBounds = p.mouseY >= y && p.mouseY <= y + barHeight;

            if (withinYBounds && (
                (p.mouseX >= xStart - edgePadding && p.mouseX <= xStart + edgePadding) ||
                (p.mouseX >= xEnd - edgePadding && p.mouseX <= xEnd + edgePadding)
            )) {
                document.body.style.cursor = "ew-resize";
            }
        });
    
        // Fix: Ensure "Now" is correctly mapped in `dateToX()`
        let nowTime = new Date().getTime();
        nowX = p.map(nowTime, timelineStart, timelineEnd, startX, p.width - startX) + offsetX;
    
        // Now line.
        p.stroke(255, 0, 0);
        p.strokeWeight(1);
        p.line(nowX, 30, nowX, p.height - 60);
        
        // Now label.
        p.fill(255, 0, 0);
        p.strokeWeight(0);
        p.textSize(14);
        p.textAlign(p.CENTER);
        p.text("Now", nowX, 20);
    };
    

    // Enable panning with mouse drag
    p.mousePressed = () => {
        isDragging = true;
        p.loop();
        dragStartX = p.mouseX - offsetX;
    };

    p.mouseReleased = () => {
        isDragging = false;
        p.noLoop();
    };

    p.mouseDragged = () => {
        if (isDragging) {
            offsetX = p.mouseX - dragStartX;
        }
    };
};

new p5(sketch);

async function fetchGanttData() {
    try {
        console.log("🔄 Fetching Gantt data from API...");

        const response = await fetch("http://localhost:8080/timeline");
        console.log("✅ API Response Received:", response);

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
                };
        });

        originalTasks = JSON.parse(JSON.stringify(tasks)); // Deep copy to track changes.

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
