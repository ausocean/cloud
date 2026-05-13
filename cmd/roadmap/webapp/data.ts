/// <reference types="vite/client" />

import { config, getCategoryEmoji } from "./config";

const API_BASE_URL = import.meta.env.VITE_API_URL || "http://localhost:8080";
const IDEA_STATUS = "idea";

export async function fetchTasks(): Promise<any[]> {
  console.log("🔄 Fetching Gantt data from API...");

  const response = await fetch(`${API_BASE_URL}/api/v1/timeline`, {
    credentials: "include",
  });
  console.log("API Response Received:", response);

  if (response.status === 401 || response.status === 403) {
    window.location.href = "/";
    return [];
  }

  if (!response.ok) {
    throw new Error(`HTTP error! Status: ${response.status}`);
  }

  const rawData = await response.json();
  console.log("📄 Raw API Data:", rawData);

  const filteredOut = new Set(config.tasks.filterOutStatuses.map(normalizeStatus));
  const defaults = config.tasks.defaults;

  const tasks = rawData
    .filter((row: any) => !filteredOut.has(normalizeStatus(row.Status || "")))
    .map((row: any) => {
      const status = row.Status || "";
      const isIdea = normalizeStatus(status) === IDEA_STATUS;
      const title = row.Title || "";
      let startDate = parseDate(row.Start || "");
      let endDate = parseDate(row.End || "");
      let categoryEmoji = getCategoryEmoji(row.Category || defaults.category);

      // Validate start and end dates.
      if (!isIdea && (!startDate || isNaN(new Date(startDate).getTime()))) {
        console.warn(`⚠️ Invalid start date for task "${row.Title}":`, startDate);
        startDate = new Date().toISOString().split("T")[0]; // Default to today.
      }

      if (!isIdea && (!endDate || isNaN(new Date(endDate).getTime()))) {
        console.warn(`⚠️ Invalid end date for task "${row.Title}":`, endDate);
        endDate = new Date(new Date(startDate).getTime() + 7 * 24 * 60 * 60 * 1000).toISOString().split("T")[0]; // Default to 7 days later
      }

      return {
        id: row.ID,
        category: row.Category,
        title,
        name: `${categoryEmoji} ${title}`, // Prepend emoji to task name.
        description: row.Description || "",
        status,
        start: startDate,
        end: endDate,
        priority: row.Priority || defaults.priority,
        owner: row.Owner || defaults.owner,
        milestone: row["Milestone Type"] === "Start Date" ? startDate : row["Milestone Type"] === "End Date" ? endDate : null,
        dependencies: row.Dependencies ? row.Dependencies.split(",").map((dep: string) => dep.trim()) : [],
      };
    });

  console.log("🛠️ Processed Tasks:", tasks);
  return tasks;
}

export async function submitTasks(tasks: any[]): Promise<void> {
  console.log("📤 Sending update request...", tasks);

  const response = await fetch(`${API_BASE_URL}/api/v1/update`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    credentials: "include",
    body: JSON.stringify({ tasks }),
  });

  if (response.status === 401 || response.status === 403) {
    window.location.href = "/";
    return;
  }

  if (!response.ok) {
    const errorText = await response.text();
    throw new Error(`Failed to update tasks: ${errorText}`);
  }

  console.log("Changes submitted successfully!");
}

// parseDate converts a "dd/mm/yyyy" date string to "yyyy-mm-dd" (ISO).
function parseDate(dateString: string): string {
  if (!dateString.trim()) return "";

  const parts = dateString.split("/");
  if (parts.length === 3) {
    return `${parts[2]}-${parts[1].padStart(2, "0")}-${parts[0].padStart(2, "0")}`;
  }
  console.warn(`⚠️ Unexpected date format: "${dateString}"`);
  return "";
}

function normalizeStatus(status: string): string {
  return status.trim().toLowerCase();
}
