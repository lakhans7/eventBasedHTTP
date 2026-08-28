-- Adds the fields needed to drive the "copy to Fiverr" gig-builder flow
-- (manual gig creation UI, see docs/api.md POST /gigs). These are additional
-- structured fields mirroring Fiverr's own gig-creation wizard (overview,
-- pricing, description & FAQ, requirements) so a seller can fill in our form
-- once and copy each field into Fiverr's real gig editor in the right order.
-- Purely local bookkeeping — none of this is sent to Fiverr (no such API).
ALTER TABLE gigs
    ADD COLUMN sub_category text,
    ADD COLUMN tags text[] NOT NULL DEFAULT '{}',
    ADD COLUMN faqs jsonb NOT NULL DEFAULT '[]',
    ADD COLUMN buyer_requirements text NOT NULL DEFAULT '';
