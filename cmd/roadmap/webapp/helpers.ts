import p5 from "p5";

export const IDEA_STATUS = "idea";
export const HALTED_STATUS = "progress made, halted";

export function isIdeaTask(task: any): boolean {
  return (
    String(task.status || "")
      .trim()
      .toLowerCase() === IDEA_STATUS
  );
}

export function isHaltedTask(task: any): boolean {
  return (
    String(task.status || "")
      .trim()
      .toLowerCase() === HALTED_STATUS
  );
}

export function isSidebarTask(task: any): boolean {
  return isIdeaTask(task) || isHaltedTask(task);
}

export function getDateBounds(items: any[]): { start: number; end: number } | null {
  const starts = items.map((task) => new Date(task.start).getTime()).filter((value) => !isNaN(value));
  const ends = items.map((task) => new Date(task.end).getTime()).filter((value) => !isNaN(value));

  if (starts.length === 0 || ends.length === 0) {
    return null;
  }

  return {
    start: Math.min(...starts),
    end: Math.max(...ends),
  };
}

export function hexToP5Color(hex: string, alpha: number, p: p5) {
  let col = p.color(hex); // Convert hex to p5 color
  col.setAlpha(alpha * 255); // p5.js alpha is 0-255, so multiply by 255
  return col;
}

export function drawArrow(p: p5, x1: number, y1: number, x2: number, y2: number) {
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

const textCache = new Map<string, string>();
export function truncateText(p: p5, value: string, maxWidth: number): string {
  if (!value) return "";
  const key = `${value}-${maxWidth}`;
  if (textCache.has(key)) return textCache.get(key)!;

  if (p.textWidth(value) <= maxWidth) {
    textCache.set(key, value);
    return value;
  }

  let truncated = value;
  while (truncated.length > 0 && p.textWidth(`${truncated}...`) > maxWidth) {
    truncated = truncated.slice(0, -1);
  }
  const result = `${truncated}...`;
  textCache.set(key, result);
  return result;
}
