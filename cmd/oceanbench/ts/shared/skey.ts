// Returns the site key from the url path. If no skey can be parsed, returns 0.
//
// The skey should be the first section of the url path:
//    /:skey/admin/broadcast/
export function getSkey(): number {
  return Number(window.location.pathname.split("/")[1]);
}
