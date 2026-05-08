export interface RoadmapTask {
  ID: string;
  Category: string;
  Title: string;
  Description: string;
  Issues?: string;
  Priority: string;
  Owner: string;
  Status: string;
  Archive?: string;
  Start?: string;
  End?: string;
  "Milestone Type"?: string;
  Dependencies?: string;
}

export interface GanttTask {
  id: string;
  name: string;
  start: string;
  end: string;
  progress: number;
  dependencies: string;
  priority: string;
}
