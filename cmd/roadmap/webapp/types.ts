export interface RoadmapTask {
  ID: string;
  Category: string;
  Title: string;
  Description: string;
  Priority: string;
  Owner: string;
  Status: string;
  Start?: string; // Optional
  End?: string; // Optional
  ActualStart?: string; // Optional
  ActualEnd?: string; // Optional
  Dependants?: string;
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
