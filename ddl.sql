CREATE LOCALITY GROUP spill_to_hdd OPTIONS (storage='ssd', ssd_to_hdd_spill_timespan='3d');

CREATE TABLE Singers (
    SingerID STRING(MAX) NOT NULL,
    FirstName STRING(1024),
    LastName STRING(1024),
    CreatedAt TIMESTAMP NOT NULL OPTIONS (allow_commit_timestamp=TRUE),
    UpdatedAt TIMESTAMP NOT NULL OPTIONS (allow_commit_timestamp=TRUE)
)
PRIMARY KEY (SingerID),
OPTIONS (locality_group = 'spill_to_hdd');

CREATE TABLE Albums (
    SingerID STRING(MAX) NOT NULL,
    AlbumID STRING(MAX) NOT NULL,
    AlbumTitle STRING(MAX),
    Price INT64 NOT NULL,
    CreatedAt TIMESTAMP NOT NULL OPTIONS (allow_commit_timestamp=TRUE),
    UpdatedAt TIMESTAMP NOT NULL OPTIONS (allow_commit_timestamp=TRUE) )
PRIMARY KEY (SingerID, AlbumID),
INTERLEAVE IN PARENT Singers ON DELETE CASCADE,
OPTIONS (locality_group = 'spill_to_hdd');