export class Subscription {
  SubscriberID: number = NaN;
  FeedID: number = NaN;
  Class: string = "";
  Prefs: string = "";
  Start: Date = new Date();
  Finish: Date = new Date();
  Renew: boolean = false;
}
