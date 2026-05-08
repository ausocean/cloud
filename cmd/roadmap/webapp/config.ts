// Per-user roadmap timeline settings: Google Sheet wiring, categories ↔ emojis,
// owner row tint colours, and priority bar colours. Shipped as
// ../roadmap.config.json (embedded by the Go backend for sheet ranges).
// Login copy, OAuth client IDs, and similar operator-owned values are not here.
import rawConfig from "../roadmap.config.json";

export interface RoadmapTimelineConfig {
  spreadsheet: {
    id: string;
    sheetName: string;
    firstDataRow: number;
    idColumn: string;
    startDateColumn: string;
    endDateColumn: string;
    headers: string[];
  };
  tasks: {
    filterOutStatuses: string[];
    defaults: {
      category: string;
      owner: string;
      priority: string;
    };
  };
  categoryEmojis: Record<string, string>;
  defaultCategoryEmoji: string;
  ownerColors: Record<string, string>;
  defaultOwnerColor: string;
  priorityColors: Record<string, string>;
  defaultPriorityColor: string;
}

export const config: RoadmapTimelineConfig = rawConfig as RoadmapTimelineConfig;

export function getCategoryEmoji(category: string): string {
  return config.categoryEmojis[category] ?? config.defaultCategoryEmoji;
}

export function getOwnerColor(owner: string): string {
  return config.ownerColors[owner] ?? config.defaultOwnerColor;
}

export function getPriorityColor(priority: string): string {
  return config.priorityColors[priority] ?? config.defaultPriorityColor;
}
