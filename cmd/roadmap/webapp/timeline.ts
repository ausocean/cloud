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
let redraw = false;

// p5.js sketch
const sketch = (p: p5) => {
    p.setup = async () => {
        await fetchGanttData();
        const canvasHeight = Math.max(p.windowHeight * 0.8, tasks.length * ySpacing + 100);
        const canvas = p.createCanvas(p.windowWidth * 0.9, canvasHeight);
        canvas.parent("canvas-container");
        p.textFont("Arial");

        console.log("🚀 Initializing Gantt Chart...");
    
        timelineStart = Math.min(...tasks.map(t => new Date(t.start).getTime()));
        timelineEnd = Math.max(...tasks.map(t => new Date(t.end).getTime()));
        redraw = true;
        p.frameRate(30);
    };
    
    p.draw = () => {
        document.body.style.cursor = "default"; // Reset cursor on each frame
        tasks.forEach((task, i) => {
            let xStart = dateToX(task.start, p) + offsetX;
            let xEnd = dateToX(task.end, p) + offsetX;
            let y = i * ySpacing + 50;

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
        p.textAlign(p.CENTER);
        p.textSize(12);
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

        // Draw colour behind tasks for task owner.
        let currentOwner = "";
        tasks.forEach((task, index) => {
            let yPos = index * ySpacing + 55;
            let backgroundColor = ownerColors[task.owner] || ownerColors["Other"];

            // Draw background color for each row.
            p.fill(backgroundColor);
            p.noStroke();
            p.rect(0, yPos - 5, p.width, ySpacing);

            // Only draw Owner name when it changes (first occurrence).
            if (task.owner !== currentOwner) {
                currentOwner = task.owner;
                let firstName = currentOwner.split(" ")[0];
                p.fill(50); // Darker text color.
                p.textSize(14);
                p.textAlign(p.LEFT);
                p.text(firstName, 5, yPos + barHeight / 2); // Left-aligned.
            }
        });

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
        });
    
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

        redraw = false;
    };
    
    // Enable panning with mouse drag
    p.mousePressed = () => {
        if (p.mouseY <= dateAreaHeight) {
            isDragging = true;
            dragStartX = p.mouseX - offsetX;
        }
    };
    p.mousePressed = () => {
        isDragging = true;
        dragStartX = p.mouseX - offsetX;
    
        selectedTask = null;
        draggingEdge = null;
    
        tasks.forEach((task) => {
            let xStart = dateToX(task.start, p) + offsetX;
            let xEnd = dateToX(task.end, p) + offsetX;
            let y = tasks.indexOf(task) * ySpacing + 50; // Get task's Y position
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
        if (isDragging && p.mouseY <= dateAreaHeight) {
            offsetX = p.mouseX - dragStartX; // Pan the timeline.
        }
    
        if (!isDragging || !selectedTask) return;
    
        let newDate = xToDate(p.mouseX - offsetX, p);
        if (draggingEdge === "start") {
            selectedTask.start = newDate;
        } else if (draggingEdge === "end") {
            selectedTask.end = newDate;
        }
    };
};

new p5(sketch);

// Convert date to X position
function dateToX(date: string, p: p5): number {
    let dateObj = new Date(date);
    return p.map(dateObj.getTime(), timelineStart, timelineEnd, startX, p.width - startX);
}

function xToDate(x: number, p: p5): string {
    let timeRange = timelineEnd - timelineStart;
    let dateMillis = timelineStart + (x / p.width) * timeRange;
    return new Date(dateMillis).toISOString().split("T")[0]; // Format as YYYY-MM-DD
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
            const errorText = await response.text(); // ✅ Read full error message
            throw new Error(`Failed to update tasks: ${errorText}`);
        }

        console.log("✅ Changes submitted successfully!");

    } catch (error) {
        console.error("❌ Error submitting changes:", error);
    }
});
