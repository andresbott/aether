/**
 * How many items one shelf of the mobile landing page shows before its "See all"
 * link takes over (`MobileBrowseView` / `BrowseShelf`). Ten is roughly three
 * swipes of a phone-width strip — enough that a shelf reads as a sample of the
 * section, few enough that every shelf's request stays cheap, since the landing
 * page fires one per section on arrival.
 */
export const BROWSE_SHELF_SIZE = 10
