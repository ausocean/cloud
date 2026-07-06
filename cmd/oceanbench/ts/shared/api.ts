export const enum method {
  GET = "GET",
  POST = "POST",
  PUT = "PUT",
  DELETE = "DELETE",
}

export function apiRequest<T>(
  url: string,
  method: method,
  data?: any,
): Promise<T> {
  return fetch(url, {
    method: method,
    body: data,
  })
    .then(async (resp) => {
      if (!resp.ok) {
        throw new Error((await resp.json()).error);
      }
      return resp.json();
    })
    .catch((err) => {
      console.error("API Error:", err);
    });
}
