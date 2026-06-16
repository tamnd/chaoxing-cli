package chaoxing

// CourseResult is one item from a Chaoxing course search.
type CourseResult struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Teacher     string `json:"teacher"`
	Institution string `json:"institution"`
	Members     int    `json:"members"`
	StartDate   string `json:"start_date"`
	EndDate     string `json:"end_date"`
	ImageURL    string `json:"image_url"`
	URL         string `json:"url"`
}

// Course is the full record for a single Chaoxing course.
type Course struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Teacher     string   `json:"teacher"`
	Institution string   `json:"institution"`
	Members     int      `json:"members"`
	Description string   `json:"description"`
	StartDate   string   `json:"start_date"`
	EndDate     string   `json:"end_date"`
	ImageURL    string   `json:"image_url"`
	Chapters    []string `json:"chapters"`
	URL         string   `json:"url"`
}
