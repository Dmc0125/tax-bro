package auth

import (
	"fmt"
	"math/rand"
)

var nouns = []string{
	"butterfly", "blanket", "carousel", "pillow", "rainbow",
	"telescope", "garden", "cookie", "mountain", "umbrella",
	"library", "dolphin", "treasure", "pencil", "lighthouse",
	"balloon", "puzzle", "cupcake", "sparrow", "backpack",
	"compass", "seashell", "sunflower", "meadow", "wagon",
	"castle", "rainbow", "feather", "island", "candle",
	"basket", "ribbon", "kitten", "puppet", "fountain",
	"cloud", "pebble", "guitar", "turtle", "bracelet",
	"pinwheel", "rainbow", "acorn", "whistle", "jellyfish",
	"locket", "snowflake", "drumstick", "starfish", "windmill",
}

var adjectives = []string{
	"Fluffy", "Sparkly", "Gentle", "Playful", "Cozy",
	"Bright", "Peaceful", "Jolly", "Merry", "Sleepy",
	"Bouncy", "Clever", "Cheerful", "Silly", "Curious",
	"Friendly", "Bubbly", "Graceful", "Snuggly", "Giggly",
	"Magical", "Sweet", "Joyful", "Warm", "Adventurous",
	"Whimsical", "Delightful", "Charming", "Dazzling", "Enchanting",
	"Fantastic", "Gleaming", "Harmonious", "Innocent", "Jubilant",
	"Lively", "Melodious", "Nurturing", "Precious", "Radiant",
	"Serene", "Tender", "Uplifting", "Vibrant", "Wonderful",
	"Zesty", "Adorable", "Breezy", "Cuddly", "Dreamy",
}

func generateRandomName(seed int64) string {
	source := rand.NewSource(int64(seed))
	rng := rand.New(source)

	adjectiveIndex := rng.Intn(len(adjectives))
	nounIndex := rng.Intn(len(nouns))

	return fmt.Sprintf("%s %s", adjectives[adjectiveIndex], nouns[nounIndex])
}
