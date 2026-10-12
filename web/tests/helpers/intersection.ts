/** Controllable IntersectionObserver: tests call `intersect()` to simulate the sentinel scrolling into view. */
export class MockIntersectionObserver {
  static instances: MockIntersectionObserver[] = [];
  readonly root = null;
  readonly rootMargin: string;
  readonly thresholds: number[] = [0];
  elements: Element[] = [];
  disconnected = false;

  constructor(
    private callback: IntersectionObserverCallback,
    options?: IntersectionObserverInit
  ) {
    this.rootMargin = options?.rootMargin ?? "0px";
    MockIntersectionObserver.instances.push(this);
  }
  observe(el: Element) {
    this.elements.push(el);
  }
  unobserve(el: Element) {
    this.elements = this.elements.filter((e) => e !== el);
  }
  disconnect() {
    this.disconnected = true;
    this.elements = [];
  }
  takeRecords(): IntersectionObserverEntry[] {
    return [];
  }
  trigger(isIntersecting: boolean) {
    const entries = this.elements.map(
      (target) => ({ isIntersecting, target, intersectionRatio: isIntersecting ? 1 : 0 }) as IntersectionObserverEntry
    );
    this.callback(entries, this as unknown as IntersectionObserver);
  }
}

/** Fires every live observer. */
export const intersectAll = (isIntersecting = true) =>
  MockIntersectionObserver.instances.filter((o) => !o.disconnected).forEach((o) => o.trigger(isIntersecting));
