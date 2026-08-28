ALTER TABLE gigs
    DROP COLUMN IF EXISTS sub_category,
    DROP COLUMN IF EXISTS tags,
    DROP COLUMN IF EXISTS faqs,
    DROP COLUMN IF EXISTS buyer_requirements;
