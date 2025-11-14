declare module "*.scss";
declare module "*.scss?inline";
declare module "*.css" {
  const content: string;
  export default content;
}
declare module "*.css?inline";
declare module "*.html";
