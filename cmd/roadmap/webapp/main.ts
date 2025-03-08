import Gantt from "frappe-gantt";
import { RoadmapTask, GanttTask } from "./types";

let gantt: Gantt | null = null;
let ganttTasks: GanttTask[] = []; // Declare globally so we can access it later

async function fetchGanttData(view_mode_select: boolean = true): Promise<void> {
    try {
        const response = await fetch("http://localhost:8080/timeline");
        const tasks: RoadmapTask[] = await response.json();

        let minDate: Date = new Date();
        let maxDate: Date = new Date();
        let isFirstIteration = true;

        ganttTasks = tasks.map(task => {
            let startDate = task.ActualStart || task.Start;
            let endDate = task.ActualEnd || task.End;
        
            if (!startDate) {
                console.warn(`Task "${task.ID}" is missing a start date. Using today's date.`);
                startDate = new Date().toISOString().split("T")[0];
            }
        
            if (!endDate) {
                endDate = new Date(new Date(startDate).setDate(new Date(startDate).getDate() + 7))
                    .toISOString()
                    .split("T")[0];
            }

            return {
                id: task.ID,
                name: task.Title,
                start: startDate,
                end: endDate,
                progress: task.Status === "Completed" ? 100 : 50,
                dependencies: task.Dependants || "",
                priority: task.Priority // ✅ Store priority for coloring later
            };
        });

        // Apply date padding
        const rangePadding = 30;
        minDate.setDate(minDate.getDate() - rangePadding);
        maxDate.setDate(maxDate.getDate() + rangePadding);

        // Initialize Gantt chart
        gantt = new Gantt("#gantt", ganttTasks, {
            header_height: 50,
            column_width: 30,
            step: 24,
            view_mode: "Day",
            bar_height: 20,
            padding: 18,
            date_format: "YYYY-MM-DD",
            language: "en",
            custom_popup_html: null,
        });

        // ✅ Wait for the Gantt chart to render before applying colors
        setTimeout(() => applyPriorityColors(), 500);

    } catch (error) {
        console.error("Error fetching Gantt data:", error);
    }
}

function getPriorityColor(priority: string): string {
    switch (priority) {
        case "P0": return "#b91c1c";  // Dark Red (Tailwind Red-700)
        case "P1": return "#d97706";  // Deeper Orange (Tailwind Amber-600)
        case "P2": return "#eab308";  // Deeper Yellow (Tailwind Yellow-600)
        case "P3": return "#16a34a";  // Medium Green (Tailwind Green-600)
        case "P4": return "#93c5fd";  // Soft Blue (Tailwind Blue-300)
        case "P5": return "#bfdbfe";  // Very Light Blue (Tailwind Blue-200)
        default: return "#e5e7eb";    // Light Gray (Neutral Default)
    }
}

function applyPriorityColors() {
    setTimeout(() => { // Ensure the Gantt chart has rendered before applying colors
        document.querySelectorAll<HTMLElement>(".bar-wrapper").forEach((wrapper) => {
            const taskId = wrapper.getAttribute("data-id");
            const task = ganttTasks.find(t => t.id === taskId);

            if (task) {
                const bar = wrapper.querySelector(".bar") as SVGRectElement;
                if (bar) {
                    bar.style.fill = getPriorityColor(task.priority); // ✅ Apply fill color directly
                }
            }
        });
    }, 500); // Delay to allow the chart to fully render
}

// Load data on page load with `view_mode_select: true`
fetchGanttData(true);
