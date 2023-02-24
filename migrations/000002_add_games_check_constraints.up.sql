ALTER TABLE games ADD CONSTRAINT games_ _check CHECK (  >= 0);
ALTER TABLE games ADD CONSTRAINT games_year_check CHECK (year BETWEEN 1888 AND date_part('year', now()));
ALTER TABLE games ADD CONSTRAINT genres_length_check CHECK (array_length(genres, 1) BETWEEN 1 AND 5);